# testclusters-go

A [golang](https://go.dev)-test compatible Kubernetes cluster test framework based on K3s.

This framework sets up and tears down Kubernetes clusters by-the-test. All you need is a [Docker](https://docker.com) environment. 

Currently, the Kubernetes API v1.28.2 is supported.

## Examples

The most minimal example would look like this, which creates a cluster and deletes all containers and networks created during the start-up:

```golang
package test

import "testing"
import "github.com/test-cluster/testclusters-go/pkg/cluster"

func TestExample(t *testing.T) {
	cluster.NewK3dCluster(t)
	// test ends and the cluster will be deleted automatically
}

```

Looking up pods from an nginx deployment:

```golang
//go:embed testdata/simpleNginxDeployment.yaml
var simpleNginxDeploymentBytes []byte

func TestExample(t *testing.T) {

	// given
	cluster := NewK3dCluster(t)
	ctx := context.Background()

	kubectl, err := cluster.CtlKube(t.Name())
	require.NoError(t, err)

	// when
	err = kubectl.ApplyWithFile(ctx, simpleNginxDeploymentBytes)
	c, err := cluster.ClientSet()
	require.NoError(t, err)

	eventualMsg := ""
	
	assert.Eventually(t, func() bool {
		list, err := c.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
			LabelSelector: "app=nginx",
		})
		if err != nil {
			eventualMsg = fmt.Sprintf("not pods with labels %s found", "app=nginx")
			return false
		}
		if len(list.Items) < 1 {
			eventualMsg = fmt.Sprintf("pod list empty for labels %s", "app=nginx")
			return false
		}

		if list.Items[0].Status.Phase != v1.PodRunning {
			eventualMsg = fmt.Sprintf("pod phase not running %s is in %s", list.Items[0].Name, list.Items[0].Status.Phase)
			return false
		}

		return true
	}, 5*time.Second, 1*time.Second)


	// then
	assert.NoError(t, err)
	assert.Equal(t, "", eventualMsg)
}
```
