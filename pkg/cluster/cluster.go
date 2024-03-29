package cluster

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/k3d-io/k3d/v5/pkg/client"
	"github.com/k3d-io/k3d/v5/pkg/config"
	configTypes "github.com/k3d-io/k3d/v5/pkg/config/types"
	"github.com/k3d-io/k3d/v5/pkg/config/v1alpha5"
	l "github.com/k3d-io/k3d/v5/pkg/logger"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
	k3dTypes "github.com/k3d-io/k3d/v5/pkg/types"
	"github.com/phayes/freeport"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/retry"

	"github.com/test-clusters/testclusters-go/pkg/naming"
)

const appName = "k8s-containers"
const DefaultNamespace = "default"

// k3s versions
// warning: k3s versions are tagged with a `+` separator before `k3s1`, but k3s images use `-`.
const (
	K3sVersion1_26 = "v1.26.2-k3s1"
	K3sVersion1_28 = "v1.28.2-k3s1"
)

type Cluster interface {
	Terminate(ctx context.Context) error
}

type K3dCluster struct {
	containerRuntime    runtimes.Runtime
	clusterConfig       *v1alpha5.ClusterConfig
	kubeConfig          *api.Config
	ClusterName         string
	AdminServiceAccount string
	clientConfig        *rest.Config
}

// NewK3dCluster creates a completely new cluster within the provided container engine. This method is the usual entry point of a test with testclusters-go.
func NewK3dCluster(t *testing.T) *K3dCluster {
	cluster := setupCluster(t)
	registerTearDown(t, cluster)

	return cluster
}

func setupCluster(t *testing.T) *K3dCluster {
	l.Log().Info("testcluster-go: Creating cluster during  ")
	var err error
	ctx := context.Background()
	cluster, err := CreateK3dCluster(ctx, "hello-world")
	if err != nil {
		t.Errorf("Unexpected error during test setup: %s\n", err)
	}
	l.Log().Info("testcluster-go: Cluster was successfully created")

	err = cluster.waitForDefaultSACreation(ctx)
	if err != nil {
		t.Errorf("failed to wait for default service account: %s", err.Error())
	}

	return cluster
}

func registerTearDown(t *testing.T, cluster *K3dCluster) {
	t.Cleanup(func() {
		l.Log().Debug("testcluster-go: Terminating cluster during test tear down")
		err := cluster.Terminate(context.Background())
		if err != nil {
			l.Log().Info("testcluster-go: Cluster was termination failed")
			t.Errorf("Unexpected error during test tear down: %s\n", err.Error())
		}
		l.Log().Info("testcluster-go: Cluster was successfully terminated")
	})
}

func createClusterConfig(ctx context.Context, clusterName string) (*v1alpha5.ClusterConfig, error) {
	freeHostPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, fmt.Errorf("could not find free port for port-forward: %w", err)
	}

	k3sRegistryYaml := `
my.company.registry":
  endpoint:
  - http://my.company.registry:5000
`
	simpleConfig := v1alpha5.SimpleConfig{
		TypeMeta: configTypes.TypeMeta{
			Kind:       "Simple",
			APIVersion: "k3d.io/v1alpha5",
		},
		ObjectMeta: configTypes.ObjectMeta{
			Name: clusterName,
		},
		Image:   fmt.Sprintf("%s:%s", k3dTypes.DefaultK3sImageRepo, K3sVersion1_28),
		Servers: 1,
		Agents:  0,
		Options: v1alpha5.SimpleConfigOptions{
			K3dOptions: v1alpha5.SimpleConfigOptionsK3d{
				Wait:    true,
				Timeout: 60 * time.Second,
			},
		},
		// allows unpublished images-under-test to be used in the cluster
		Registries: v1alpha5.SimpleConfigRegistries{
			Create: &v1alpha5.SimpleConfigRegistryCreateConfig{
				//Name:	fmt.Sprintf("%s-%s-registry", k3dTypes.DefaultObjectNamePrefix, newCluster.Name),
				// Host:    "0.0.0.0",
				HostPort: k3dTypes.DefaultRegistryPort, // alternatively the string "random"
				// Image:    fmt.Sprintf("%s:%s", k3dTypes.DefaultRegistryImageRepo, k3dTypes.DefaultRegistryImageTag),
				Proxy: k3dTypes.RegistryProxy{
					RemoteURL: "https://registry-1.docker.io",
					Username:  "",
					Password:  "",
				},
			},
			Config: k3sRegistryYaml,
		},
		ExposeAPI: v1alpha5.SimpleExposureOpts{
			HostPort: strconv.Itoa(freeHostPort),
		},
	}

	if err := config.ProcessSimpleConfig(&simpleConfig); err != nil {
		return nil, fmt.Errorf("processing simple cluster config failed: %w", err)
	}

	clusterConfig, err := config.TransformSimpleToClusterConfig(ctx, runtimes.SelectedRuntime, simpleConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to transform cluster config: %w", err)
	}

	l.Log().Debugf("===== used cluster config =====\n%#v\n===== =====", clusterConfig)

	clusterConfig, err = config.ProcessClusterConfig(*clusterConfig)
	if err != nil {
		if err != nil {
			return nil, fmt.Errorf("processing cluster config failed: %w", err)
		}
	}

	if err = config.ValidateClusterConfig(ctx, runtimes.SelectedRuntime, *clusterConfig); err != nil {
		if err != nil {
			return nil, fmt.Errorf("failed cluster config validation: %w", err)
		}
	}

	return clusterConfig, nil
}

// TODO allow the user to overwrite our ClusterConfig with her own
// func CreateK3dClusterWithConfig() ...

// CreateK3dCluster creates a completely new K8s cluster with an optional clusterNamePrefix.
func CreateK3dCluster(ctx context.Context, clusterNamePrefix string) (*K3dCluster, error) {
	containerRuntime := runtimes.SelectedRuntime

	clusterName := naming.MustGenerateK8sName(clusterNamePrefix)
	cluster := &K3dCluster{
		containerRuntime: containerRuntime,
		ClusterName:      clusterName,
	}

	var err error
	cluster.clusterConfig, err = createClusterConfig(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	err = client.ClusterRun(ctx, containerRuntime, cluster.clusterConfig)
	if err != nil {
		return cluster, handleStartError(ctx, cluster, err)
	}

	cluster.kubeConfig, err = client.KubeconfigGet(ctx, containerRuntime, &cluster.clusterConfig.Cluster)
	if err != nil {
		return cluster, handleStartError(ctx, cluster, err)
	}
	l.Log().Debugf("testcluster-go: ===== retrieved kube config ====\n%#v\n===== =====", cluster.kubeConfig)

	sa, err := createDefaultRBACForSA(ctx, cluster)
	if err != nil {
		return cluster, handleStartError(ctx, cluster, fmt.Errorf("failed to create default RBAC for SA: %w", err))
	}
	cluster.AdminServiceAccount = sa

	return cluster, nil
}

func createDefaultRBACForSA(ctx context.Context, c *K3dCluster) (string, error) {
	const globalGalacticClusterAdminSuffix = "ford-prefect"

	clientSet, err := c.ClientSet()
	if err != nil {
		return "", err
	}

	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sa-" + globalGalacticClusterAdminSuffix,
			Namespace: DefaultNamespace,
			Labels:    map[string]string{"k3s.creator": appName},
		},
	}

	sa, err = clientSet.CoreV1().ServiceAccounts(DefaultNamespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cr-" + globalGalacticClusterAdminSuffix,
			Namespace: DefaultNamespace,
			Labels:    map[string]string{"k3s.creator": appName},
		},
		Rules: []rbacv1.PolicyRule{
			{
				Verbs:     []string{rbacv1.VerbAll},
				APIGroups: []string{rbacv1.APIGroupAll},
				Resources: []string{rbacv1.ResourceAll},
			},
		},
	}

	clusterRole, err = clientSet.RbacV1().ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	clusterRoleBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "crb-" + globalGalacticClusterAdminSuffix,
			Namespace: DefaultNamespace,
			Labels:    map[string]string{"k3s.creator": appName},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     clusterRole.Name,
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      sa.Name,
				Namespace: DefaultNamespace,
			},
		},
	}

	clusterRoleBinding, err = clientSet.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return sa.Name, nil
}

func (c *K3dCluster) waitForDefaultSACreation(ctx context.Context) error {
	errRetriable := true

	err := retry.OnError(wait.Backoff{
		Steps:    20,
		Duration: 500 * time.Millisecond,
		Factor:   1.0,
		Jitter:   0.1,
	}, func(err error) bool {
		return errRetriable
	}, func() error {
		clientset, err := c.ClientSet()
		if err != nil {
			errRetriable = false
			return err
		}

		_, err = clientset.CoreV1().ServiceAccounts(DefaultNamespace).Get(ctx, "default", metav1.GetOptions{})
		if err != nil {
			l.Log().Info("testcluster-go: no default SA found")
			return err
		}
		l.Log().Info("testcluster-go: found default SA")
		return nil
	})

	if err != nil {
		return fmt.Errorf("waited too long for default SA: %w", err)
	}
	return nil
}

func handleStartError(ctx context.Context, cluster *K3dCluster, err error) error {
	err2 := cluster.Terminate(ctx)
	if err2 != nil {
		l.Log().Errorf("Another error '%s' occurred while terminating the cluster due to the original error (you may want to clean-up the container landscape): %s", err2.Error(), err)
	}

	return err
}

// Terminate shuts down the configured cluster.
func (c *K3dCluster) Terminate(ctx context.Context) error {
	err := client.ClusterDelete(ctx, c.containerRuntime, &c.clusterConfig.Cluster, k3dTypes.ClusterDeleteOpts{})
	if err != nil {
		return err
	}

	return nil
}

// ClientSet returns a K8s clientset which allows to interoperate with the cluster K8s API.
func (c *K3dCluster) ClientSet() (*kubernetes.Clientset, error) {
	if c.kubeConfig == nil {
		panic("cluster kubeConfig went unexpectedly nil")
	}
	intermediateConfig := clientcmd.NewDefaultClientConfig(*c.kubeConfig, nil)
	clientConfig, err := intermediateConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	c.clientConfig = clientConfig

	clientSet, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	return clientSet, nil
}

func (c *K3dCluster) CtlKube(fieldManager string) (*YamlApplier, error) {
	yamlApplier, err := NewYamlApplier(c.clientConfig, fieldManager, DefaultNamespace)
	if err != nil {
		return nil, fmt.Errorf("ctlkube call failed: %w", err)
	}
	return yamlApplier, nil
}

type Lookout struct {
	t *testing.T
	c kubernetes.Interface
}

func (l *Lookout) Pods(namespace string) *PodListSelector {
	return &PodListSelector{
		podClient: l.c.CoreV1().Pods(namespace),
	}
}

func (l *Lookout) Pod(namespace, name string) *PodSelector {
	return &PodSelector{
		podClient:   l.c.CoreV1().Pods(namespace),
		eventClient: l.c.CoreV1().Events(namespace),
		name:        name,
	}
}

func (c *K3dCluster) Lookout(t *testing.T) *Lookout {
	clientSet, err := c.ClientSet()
	if err != nil {
		t.Errorf("could not build clientSet for cluster: %s", err.Error())
	}

	return &Lookout{
		t: t,
		c: clientSet,
	}
}
