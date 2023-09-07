package cluster

import (
	"context"
	_ "embed"
	"testing"

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
	l.Log().Info("===== =====")
	l.Log().Infof("apply yaml bytes with result %v", err)
	l.Log().Info("===== =====")

	// then
	assert.NoError(t, err)
}
