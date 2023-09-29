package cluster

import (
	"context"
	_ "embed"
	v1 "k8s.io/api/core/v1"
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
	//c, err := cluster.ClientSet()
	require.NoError(t, err)

	pods := cluster.Lookout(t).Pods("default").ByLabels("app=nginx")
	assert.EventuallyWithT(t, func(collectT *assert.CollectT) {
		err := pods.Len(ctx, 3)
		if err != nil {
			collectT.Errorf("%w", err)
		}
	}, 10*time.Second, 1*time.Second)

	assert.EventuallyWithT(t, func(collectT *assert.CollectT) {
		err := pods.StatusPhase(ctx, v1.PodRunning)
		if err != nil {
			collectT.Errorf("%w", err)
		}
	}, 60*time.Second, 1*time.Second)

	l.Log().Info("===== =====")
	l.Log().Infof("apply yaml bytes with result %v", err)
	l.Log().Info("===== =====")

	// then
	assert.NoError(t, err)
}

var defaultTimeout = 20 * time.Second

func Within(t *testing.T, yourAssert func()) {

}
