package cluster

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
)

// KubeCtl provides a pod with kubectl access to the cluster.
type KubeCtl struct {
	pod             *v1.Pod
	commandExecutor *defaultCommandExecutor
}

// ApplyWithFile copies the yaml bytes into a file and calls `kubectl apply -f`.
func (k *KubeCtl) ApplyWithFile(ctx context.Context, yamlBytes []byte) error {
	copyFileCommand := fmt.Sprintf("echo '%s' > /tmp/resource", string(yamlBytes))
	cmd := NewShellCommand("sh", "-c", copyFileCommand)
	buf, err := k.commandExecutor.ExecCommandForPod(ctx, k.pod, cmd, "running")
	if err != nil {
		return err
	}
	fmt.Println(buf)
	// TODO continue with trying to call this in a test and check for
	return nil
}
