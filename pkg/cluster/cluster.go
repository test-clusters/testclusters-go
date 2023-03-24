package cluster

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/k3d-io/k3d/v5/pkg/client"
	"github.com/k3d-io/k3d/v5/pkg/config"
	configTypes "github.com/k3d-io/k3d/v5/pkg/config/types"
	"github.com/k3d-io/k3d/v5/pkg/config/v1alpha4"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
	k3dTypes "github.com/k3d-io/k3d/v5/pkg/types"
	"github.com/phayes/freeport"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	"github.com/ppxl/testclusters-go/pkg/naming"
)

const appName = "k8s-containers"

// k3s versions
const (
	K3sVersion1_26 = "v1.26.2-k3s1"
)

type Cluster interface {
	Terminate(ctx context.Context) error
}

type K3dCluster struct {
	containerRuntime    runtimes.Runtime
	clusterConfig       *v1alpha4.ClusterConfig
	kubeConfig          *api.Config
	ClusterName         string
	AdminServiceAccount string
}

func createClusterConfig(ctx context.Context, clusterName string) (*v1alpha4.ClusterConfig, error) {
	freeHostPort, err := freeport.GetFreePort()
	if err != nil {
		return nil, fmt.Errorf("could not find free port for port-forward: %w", err)
	}

	k3sRegistryYaml := `
my.company.registry":
  endpoint:
  - http://my.company.registry:5000
`
	simpleConfig := v1alpha4.SimpleConfig{
		TypeMeta: configTypes.TypeMeta{
			Kind:       "Simple",
			APIVersion: "k3d.io/v1alpha4",
		},
		ObjectMeta: configTypes.ObjectMeta{
			Name: clusterName,
		},
		Image:   fmt.Sprintf("%s:%s", k3dTypes.DefaultK3sImageRepo, K3sVersion1_26),
		Servers: 1,
		Agents:  0,
		Options: v1alpha4.SimpleConfigOptions{
			K3dOptions: v1alpha4.SimpleConfigOptionsK3d{
				Wait:    true,
				Timeout: 60 * time.Second,
			},
		},
		// allows unpublished images-under-test to be used in the cluster
		Registries: v1alpha4.SimpleConfigRegistries{
			Create: &v1alpha4.SimpleConfigRegistryCreateConfig{
				// Name:	fmt.Sprintf("%s-%s-registry", k3dTypes.DefaultObjectNamePrefix, newCluster.Name),
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
		ExposeAPI: v1alpha4.SimpleExposureOpts{
			HostPort: strconv.Itoa(freeHostPort),
		},
	}

	if err := config.ProcessSimpleConfig(&simpleConfig); err != nil {
		return nil, err
	}

	clusterConfig, err := config.TransformSimpleToClusterConfig(ctx, runtimes.SelectedRuntime, simpleConfig)
	if err != nil {
		return nil, err
	}

	clusterConfig, err = config.ProcessClusterConfig(*clusterConfig)
	if err != nil {
		if err != nil {
			return nil, err
		}
	}

	if err = config.ValidateClusterConfig(ctx, runtimes.SelectedRuntime, *clusterConfig); err != nil {
		if err != nil {
			return nil, err
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

	sa, err := createDefaultRBACForSA(ctx, cluster)
	cluster.AdminServiceAccount = sa

	return cluster, nil
}

func createDefaultRBACForSA(ctx context.Context, c *K3dCluster) (string, error) {
	const targetNamespace = "default"
	const globalGalacticClusterAdminSuffix = "ford-prefect"

	clientSet, err := c.ClientSet()
	if err != nil {
		return "", err
	}

	sa := &v1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sa-" + globalGalacticClusterAdminSuffix,
			Namespace: targetNamespace,
			Labels:    map[string]string{"k3s.creator": appName},
		},
	}

	sa, err = clientSet.CoreV1().ServiceAccounts(targetNamespace).Create(ctx, sa, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cr-" + globalGalacticClusterAdminSuffix,
			Namespace: targetNamespace,
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
			Namespace: targetNamespace,
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
				Namespace: targetNamespace,
			},
		},
	}

	clusterRoleBinding, err = clientSet.RbacV1().ClusterRoleBindings().Create(ctx, clusterRoleBinding, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}

	return sa.Name, nil
}

func handleStartError(ctx context.Context, cluster *K3dCluster, err error) error {
	err2 := cluster.Terminate(ctx)
	if err2 != nil {
		fmt.Printf("Another error '%s' occurred during an error: %s", err2.Error(), err)
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
	intermediateConfig := clientcmd.NewDefaultClientConfig(*c.kubeConfig, nil)
	clientConfig, err := intermediateConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}

	return clientSet, nil
}

func (c *K3dCluster) Kubectl(ctx context.Context) (*KubeCtl, error) {
	clientSet, err := c.ClientSet()
	if err != nil {
		return nil, err
	}

	const kubectlPodName = "kubectl-pod"
	kubeCtlPod, err := clientSet.CoreV1().Pods("default").Get(ctx, kubectlPodName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, err
	}

	if apierrors.IsNotFound(err) {
		trueish := true
		kubeCtlPod = &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      kubectlPodName,
				Namespace: "default",
				Labels:    map[string]string{"k3s.creator": appName},
			},
			Spec: v1.PodSpec{
				Containers: []v1.Container{{
					Name:    kubectlPodName,
					Image:   "bitnami/kubectl:1.26.2",
					Command: []string{"sleep infinity"},
				}},
				AutomountServiceAccountToken: &trueish,
				ServiceAccountName:           c.AdminServiceAccount,
			},
			Status: v1.PodStatus{},
		}

		kubeCtlPod, err = clientSet.CoreV1().Pods("default").Create(ctx, kubeCtlPod, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
	}

	coreV1Client := clientSet.CoreV1().RESTClient()

	return &KubeCtl{
		pod:             kubeCtlPod,
		commandExecutor: NewCommandExecutor(clientSet, coreV1Client),
	}, nil
}
