package cluster

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	typecorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type PodSelector struct {
	podClient   typecorev1.PodInterface
	eventClient typecorev1.EventInterface
	name        string
}

func (ps *PodSelector) Raw(ctx context.Context) (*corev1.Pod, error) {
	return ps.podClient.Get(ctx, ps.name, metav1.GetOptions{})
}

func (ps *PodSelector) Events(ctx context.Context, fieldSelectors ...string) (*corev1.EventList, error) {
	joinedSelector := strings.Join(append(fieldSelectors, fmt.Sprintf("involvedObject.name=%s", ps.name)), ",")
	return ps.eventClient.List(ctx, metav1.ListOptions{
		FieldSelector: joinedSelector,
		TypeMeta:      metav1.TypeMeta{Kind: "Pod"},
	})
}

func (ps *PodSelector) Logs(ctx context.Context) ([]byte, error) {
	podLogOpts := &corev1.PodLogOptions{}
	logReq := ps.podClient.GetLogs(ps.name, podLogOpts)
	result := logReq.Do(ctx)
	if result.Error() != nil {
		return []byte{}, result.Error()
	}

	raw, err := result.Raw()
	if err != nil {
		return nil, err
	}

	return raw, nil
}
