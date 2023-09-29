package cluster_test

import (
	"context"
	_ "embed"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"

	"github.com/test-clusters/testclusters-go/pkg/cluster"
)

//go:embed testdata/simpleNginxDeployment.yaml
var simpleNginxDeploymentBytes []byte

func TestIntegration(t *testing.T) {

	// given
	cl := cluster.NewK3dCluster(t)
	ctx := context.Background()

	kubectl, err := cl.CtlKube(t.Name())
	require.NoError(t, err)

	// when
	err = kubectl.ApplyWithFile(ctx, simpleNginxDeploymentBytes)
	//c, err := cluster.ClientSet()
	require.NoError(t, err)

	pods := cl.Lookout(t).Pods("default").ByLabels("app=nginx")
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

	// then
	assert.NoError(t, err)
}
