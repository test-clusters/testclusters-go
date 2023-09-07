package cluster

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	l "github.com/k3d-io/k3d/v5/pkg/logger"
	"net/url"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/client-go/util/retry"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// ShellCommand represents all necessary arguments to execute a command inside a container.
type ShellCommand struct {
	// command states the actual executable that is supposed to be executed in the container.
	command string
	// args contains any parameters, switches etc. that the command needs to run properly.
	args []string
}

func (sc *ShellCommand) CommandWithArgs() []string {
	return append([]string{sc.command}, sc.args...)
}

func (sc *ShellCommand) String() string {
	result := []string{sc.command}
	return strings.Join(append(result, sc.args...), " ")
}

// NewShellCommand creates a new ShellCommand. While the command is mandatory, there can be zero to n command arguments.
func NewShellCommand(command string, args ...string) ShellCommand {
	return ShellCommand{command: command, args: args}
}

// stateError is returned when a specific resource (pod/dogu) does not meet the requirements for the exec.
type stateError struct {
	sourceError error
	resource    metav1.Object
}

// Report returns the error in string representation
func (e *stateError) Error() string {
	return fmt.Sprintf("resource does not meet requirements for exec: %v, source error: %s", e.resource.GetName(), e.sourceError.Error())
}

// Requeue determines if the current dogu operation should be requeue when this error was responsible for its failure
func (e *stateError) Requeue() bool {
	return true
}

// maxTries controls the maximum number of waiting intervals between tries when getting an error that is recoverable
// during command execution.
var maxTries = 20

// commandExecutor is the unit to execute commands in a dogu
type defaultCommandExecutor struct {
	clientSet              kubernetes.Interface
	coreV1RestClient       rest.Interface
	commandExecutorCreator func(config *rest.Config, method string, url *url.URL) (remotecommand.Executor, error)
}

// NewCommandExecutor creates a new instance of NewCommandExecutor
func NewCommandExecutor(clientSet kubernetes.Interface, coreV1RestClient rest.Interface) *defaultCommandExecutor {
	return &defaultCommandExecutor{
		clientSet: clientSet,
		// the rest clientSet COULD be generated from the clientSet but makes harder to test, so we source it additionally
		coreV1RestClient:       coreV1RestClient,
		commandExecutorCreator: remotecommand.NewSPDYExecutor,
	}
}

// ExecCommandForPod execs a command in a given pod. This method executes a command on an arbitrary pod that can be
// identified by its pod name.
func (ce *defaultCommandExecutor) ExecCommandForPod(ctx context.Context, pod *corev1.Pod, command ShellCommand, expectedStatus string) (*bytes.Buffer, error) {
	err := ce.waitForPodToHaveExpectedStatus(ctx, pod, expectedStatus)
	if err != nil {
		return nil, fmt.Errorf("an error occurred while waiting for pod %s to have status %s: %w", pod.Name, expectedStatus, err)
	}

	req := ce.getCreateExecRequest(pod, command)
	exec, err := ce.commandExecutorCreator(ctrl.GetConfigOrDie(), "POST", req.URL())
	if err != nil {
		return nil, &stateError{
			sourceError: fmt.Errorf("failed to create new spdy executor: %w", err),
			resource:    pod,
		}
	}

	return ce.streamCommandToPod(ctx, exec, command, pod)
}

func (ce *defaultCommandExecutor) streamCommandToPod(
	ctx context.Context,
	exec remotecommand.Executor,
	command ShellCommand,
	pod *corev1.Pod,
) (*bytes.Buffer, error) {
	logger := log.FromContext(ctx)

	var err error
	buffer := bytes.NewBuffer([]byte{})
	bufferErr := bytes.NewBuffer([]byte{})

	err = retry.OnError(wait.Backoff{
		Duration: 1500 * time.Millisecond,
		Factor:   1.5,
		Jitter:   0,
		Steps:    maxTries,
		Cap:      3 * time.Minute,
	}, func(err error) bool {
		return strings.Contains(err.Error(), "error dialing backend: EOF")
	}, func() error {
		err = exec.StreamWithContext(ctx, remotecommand.StreamOptions{
			Stdout: buffer,
			Stderr: bufferErr,
			Tty:    false,
		})
		if err != nil {
			// ignore this error and retry again instead since the container did not receive the command
			if strings.Contains(err.Error(), "error dialing backend: EOF") {
				logger.Error(err, fmt.Sprintf("Error executing '%s' in pod %s. Trying again.", command, pod.Name))
			}
		}
		return err
	})
	if err != nil {
		return nil, &stateError{
			sourceError: fmt.Errorf("error streaming command to pod; out: '%s': errOut: '%s': %w", buffer, bufferErr, err),
			resource:    pod,
		}
	}

	return buffer, nil
}

func (ce *defaultCommandExecutor) waitForPodToHaveExpectedStatus(ctx context.Context, pod *corev1.Pod, expectedPodStatus string) error {
	var err error
	err = retry.OnError(wait.Backoff{
		Duration: 1500 * time.Millisecond,
		Factor:   1.5,
		Jitter:   0,
		Steps:    maxTries,
		Cap:      3 * time.Minute}, TestableRetryFunc, func() error {
		pod, err = ce.clientSet.CoreV1().Pods(pod.Namespace).Get(ctx, pod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		return podHasStatus(pod, expectedPodStatus)
	})

	return err
}

func podHasStatus(pod *corev1.Pod, expectedPodStatus string) error {
	l.Log().Info("===== =====")
	l.Log().Infof("podHasStatus %#v", pod)
	l.Log().Info("===== =====")

	switch expectedPodStatus {
	case "started":
		if pod.Status.Phase == corev1.PodRunning {
			return nil
		}
	case "ready":
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.ContainersReady && condition.Status == corev1.ConditionTrue {
				return nil
			}
		}
	default:
		return fmt.Errorf("unsupported pod status: %s", expectedPodStatus)
	}

	l.Log().Info("===== =====")
	l.Log().Infof("expectedPodStatus status %s not fulfilled", expectedPodStatus)
	l.Log().Info("===== =====")
	return &TestableRetrierError{Err: fmt.Errorf("expectedPodStatus status %s not fulfilled", expectedPodStatus)}
}

// TestableRetryFunc returns true if the returned error is a TestableRetrierError and indicates that an action should be tried until the retrier hits its limit.
var TestableRetryFunc = func(err error) bool {
	var testableRetrierError *TestableRetrierError
	ok := errors.As(err, &testableRetrierError)
	return ok
}

// TestableRetrierError marks errors that indicate that a previously executed action should be retried with again. It must wrap an existing error.
type TestableRetrierError struct {
	Err error
}

// Error returns the error's string representation.
func (tre *TestableRetrierError) Error() string {
	return tre.Err.Error()
}
func (ce *defaultCommandExecutor) getCreateExecRequest(pod *corev1.Pod, command ShellCommand) *rest.Request {
	return ce.coreV1RestClient.Post().
		Resource("pods").
		Name(pod.Name).
		Namespace(pod.Namespace).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Command: command.CommandWithArgs(),
			Stdin:   false,
			Stdout:  true,
			Stderr:  true,
			// Note: if the TTY is set to true shell commands may emit ANSI codes into the stdout
			TTY: false,
		}, scheme.ParameterCodec)
}
