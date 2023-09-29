package cluster

import (
	"context"
	_ "embed"
	"fmt"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"testing"
	"time"

	l "github.com/k3d-io/k3d/v5/pkg/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/simpleNginxDeployment.yaml
var simpleNginxDeploymentBytes []byte

func TestExample(t *testing.T) {

	cluster := NewK3dCluster(t)

	// given
	ctx := context.Background()
	l.Log().Info("===== =====")
	l.Log().Info("get kubectl")
	l.Log().Info("===== =====")
	kubectl, err := cluster.CtlKube(t.Name())
	require.NoError(t, err)

	// when
	l.Log().Info("===== =====")
	l.Log().Info("apply yaml bytes")
	l.Log().Info("===== =====")
	err = kubectl.ApplyWithFile(ctx, simpleNginxDeploymentBytes)
	c, err := cluster.ClientSet()
	require.NoError(t, err)

	leMsg := "asdf"
	assert.Eventuallyf(t, func() bool {
		list, err := c.CoreV1().Pods("default").List(ctx, metav1.ListOptions{
			LabelSelector: "app=nginx",
		})
		if err != nil {
			leMsg = fmt.Sprintf("not pods with labels %s found", "app=nginx")
			println("not yet app=nginx seletect")
			return false
		}
		if len(list.Items) < 1 {
			leMsg = fmt.Sprintf("pod list empty for labels %s", "app=nginx")
			println("pod list empty app=nginx")
			return false
		}

		if list.Items[0].Status.Phase != v1.PodRunning {
			leMsg = fmt.Sprintf("pod phase not running %s is in %s", list.Items[0].Name, list.Items[0].Status.Phase)
			println("pod phase not running app=nginx", list.Items[0].Name)
			return false
		}

		return true
	}, 5*time.Second, 1*time.Second, leMsg)
	assert.Equal(t, "qwer", leMsg)

	l.Log().Info("===== =====")
	l.Log().Infof("apply yaml bytes with result %v", err)
	l.Log().Info("===== =====")

	// then
	assert.NoError(t, err)
}

var defaultTimeout = 20 * time.Second

func Within(t *testing.T, yourAssert func()) {

}
