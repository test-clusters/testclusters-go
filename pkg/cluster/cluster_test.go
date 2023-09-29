package cluster_test

import (
	"context"
	_ "embed"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/test-clusters/testclusters-go/pkg/cluster"
)

//go:embed testdata/simpleNginxDeployment.yaml
var simpleNginxDeploymentBytes []byte

//go:embed testdata/simpleEchoPod.yaml
var simpleEchoPodBytes []byte

func TestIntegration(t *testing.T) {
	// given
	cl := cluster.NewK3dCluster(t)
	ctx := context.Background()

	kubectl, err := cl.CtlKube(t.Name())
	require.NoError(t, err)

	// when
	err = kubectl.ApplyWithFile(ctx, simpleNginxDeploymentBytes)
	require.NoError(t, err)
	err = kubectl.ApplyWithFile(ctx, simpleEchoPodBytes)
	require.NoError(t, err)

	lookout := cl.Lookout(t)

	pods := lookout.Pods(cluster.DefaultNamespace).ByLabels("app=nginx").ByFieldSelector("status.phase=Running").List()
	assert.EventuallyWithT(t, func(collectT *assert.CollectT) {
		err := pods.Len(ctx, 3)
		if err != nil {
			collectT.Errorf("%w", err)
		}
	}, 60*time.Second, 1*time.Second)

	podList, err := pods.Raw(ctx)
	require.NoError(t, err)

	events, err := cl.Lookout(t).Pod(cluster.DefaultNamespace, podList.Items[0].Name).Events(ctx)
	require.NoError(t, err)
	fmt.Printf("%#v", events)

	actualLogs, err := cl.Lookout(t).Pod(cluster.DefaultNamespace, "echo-pod").Logs(ctx)
	require.NoError(t, err)

	assert.Equal(t, "hello world\n", string(actualLogs))

	// then
	assert.NoError(t, err)
}
