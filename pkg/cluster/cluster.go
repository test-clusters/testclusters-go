package cluster

import (
	"context"
	"fmt"
	"time"

	"github.com/k3d-io/k3d/v5/pkg/client"
	"github.com/k3d-io/k3d/v5/pkg/config"
	configTypes "github.com/k3d-io/k3d/v5/pkg/config/types"
	"github.com/k3d-io/k3d/v5/pkg/config/v1alpha4"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"
	"github.com/k3d-io/k3d/v5/pkg/types"
	"k8s.io/client-go/tools/clientcmd/api"
)

type Cluster interface {
	Terminate(ctx context.Context) error
}

type K3dCluster struct {
	containerRuntime runtimes.Runtime
	clusterConfig    *v1alpha4.ClusterConfig
	kubeConfig       *api.Config
}

func createClusterConfig(ctx context.Context) (*v1alpha4.ClusterConfig, error) {
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
			Name: "to-be-generated",
		},
		// Image:   fmt.Sprintf("%s:%s", types.DefaultK3sImageRepo, version.K3sVersion),
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
				// Name:	fmt.Sprintf("%s-%s-registry", k3d.DefaultObjectNamePrefix, newCluster.Name),
				// Host:    "0.0.0.0",
				HostPort: types.DefaultRegistryPort, // alternatively the string "random"
				// Image:   fmt.Sprintf("%s:%s", k3d.DefaultRegistryImageRepo, k3d.DefaultRegistryImageTag),
				Proxy: types.RegistryProxy{
					RemoteURL: "https://registry-1.docker.io",
					Username:  "",
					Password:  "",
				},
			},
			Config: k3sRegistryYaml,
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

func CreateK3dCluster(ctx context.Context) (*K3dCluster, error) {
	containerRuntime := runtimes.SelectedRuntime

	cluster := &K3dCluster{
		containerRuntime: containerRuntime,
	}

	var err error
	cluster.clusterConfig, err = createClusterConfig(ctx)
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

	return cluster, nil
}

func handleStartError(ctx context.Context, cluster *K3dCluster, err error) error {
	err2 := cluster.Terminate(ctx)
	if err2 != nil {
		fmt.Printf("Another error '%s' occurred during an error: %s", err2.Error(), err)
	}

	return err
}

func (c *K3dCluster) Terminate(ctx context.Context) error {
	err := client.ClusterDelete(ctx, c.containerRuntime, &c.clusterConfig.Cluster, types.ClusterDeleteOpts{})
	if err != nil {
		return err
	}

	return nil
}
