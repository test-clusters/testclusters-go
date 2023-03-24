package cluster

import (
	"context"
	_ "embed"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var cluster *K3dCluster

func TestMain(m *testing.M) {
	ctx := context.Background()
	var exitCode = 1
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Unexpected panic during test")
		}
		err := tearDown(ctx)
		if err != nil {
			log.Printf("Unexpected error during test tear down: %s\n", err)
			return
		}
		os.Exit(exitCode)
	}()

	err := setup(ctx)
	if err != nil {
		log.Printf("Unexpected error during test setup: %s\n", err)
		return
	}

	exitCode = m.Run()

}

func setup(ctx context.Context) error {
	var err error
	cluster, err = CreateK3dCluster(ctx, "hello-world")
	if err != nil {
		return err
	}
	return nil
}

func tearDown(ctx context.Context) error {
	err := cluster.Terminate(ctx)
	if err != nil {
		return err
	}
	return nil
}

//go:embed testdata/simpleNginxDeployment.yaml
var simpleNginxDeploymentBytes []byte

func TestExample(t *testing.T) {
	// given
	ctx := context.Background()
	kubectl, err := cluster.Kubectl(ctx)
	require.NoError(t, err)

	// when
	err = kubectl.ApplyWithFile(ctx, simpleNginxDeploymentBytes)

	// then
	assert.NoError(t, err)
}
