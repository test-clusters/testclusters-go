package cluster

import (
	"context"
	"time"

	"github.com/k3d-io/k3d/v5/pkg/client"
	"github.com/k3d-io/k3d/v5/pkg/config"
	configTypes "github.com/k3d-io/k3d/v5/pkg/config/types"
	"github.com/k3d-io/k3d/v5/pkg/config/v1alpha4"
	"github.com/k3d-io/k3d/v5/pkg/runtimes"

	"k8s.io/client-go/tools/clientcmd/api"
)

type Cluster interface {
	Terminate() error
}

type K3dCluster struct {
	kubeConfig *api.Config
}

func createClusterConfig(ctx context.Context) (*v1alpha4.ClusterConfig, error) {
	simpleConfig := v1alpha4.SimpleConfig{
		TypeMeta: configTypes.TypeMeta{
			Kind:       "Simple",
			APIVersion: "k3d.io/v1alpha4",
		},
		ObjectMeta: configTypes.ObjectMeta{
			Name: "to-be-generated",
		},
		Servers: 1,
		Agents:  0,
		Options: v1alpha4.SimpleConfigOptions{
			K3dOptions: v1alpha4.SimpleConfigOptionsK3d{
				Wait:    true,
				Timeout: 60 * time.Second,
			},
		},
	}

	clusterConfig, err := config.TransformSimpleToClusterConfig(ctx, runtimes.SelectedRuntime, simpleConfig)
	if err != nil {
		return nil, err
	}

	return clusterConfig, nil
}

func CreateK3dCluster(ctx context.Context) (*K3dCluster, error) {
	clusterConfig, err := createClusterConfig(ctx)
	if err != nil {
		return nil, err
	}

	err = client.ClusterRun(ctx, runtimes.SelectedRuntime, clusterConfig)
	if err != nil {
		return nil, err
	}

	kubeConfig, err := client.KubeconfigGet(ctx, runtimes.SelectedRuntime, &clusterConfig.Cluster)
	if err != nil {
		return nil, err
	}

	return &K3dCluster{
		kubeConfig: kubeConfig,
	}, nil
}
