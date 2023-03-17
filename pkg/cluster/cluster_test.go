package cluster

import (
	"context"
	_ "embed"
	"log"
	"os"
	"testing"
)

var cluster *K3dCluster

func TestMain(m *testing.M) {
	ctx := context.Background()
	var exitCode = 1
	defer func() { os.Exit(exitCode) }()

	err := setup(ctx)
	if err != nil {
		log.Printf("Unexpected error during test setup: %s\n", err)
		return
	}

	exitCode = m.Run()

	err = tearDown(ctx)
	if err != nil {
		log.Printf("Unexpected error during test tear down: %s\n", err)
		return
	}
}

func setup(ctx context.Context) error {
	var err error
	cluster, err = CreateK3dCluster(ctx)
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
	// kubectl, _ := cluster.Kubectl()
	// err := kubectl.Apply(simpleNginxDeploymentBytes)
	// if err != nil {
	// 	panic("eaassfafarrggh!!!! " + err.Error())
	// }
}
