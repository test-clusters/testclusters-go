package cluster

import (
	"context"
	"fmt"
	"strings"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type PodSelector struct {
	podClient   corev1.PodInterface
	eventClient corev1.EventInterface
	name        string
}

func (ps *PodSelector) Raw(ctx context.Context) (*v1.Pod, error) {
	return ps.podClient.Get(ctx, ps.name, metav1.GetOptions{})
}

func (ps *PodSelector) Events(ctx context.Context, fieldSelectors ...string) (*v1.EventList, error) {
	joinedSelector := strings.Join(append(fieldSelectors, fmt.Sprintf("involvedObject.name=%s", ps.name)), ",")
	return ps.eventClient.List(ctx, metav1.ListOptions{
		FieldSelector: joinedSelector,
		TypeMeta:      metav1.TypeMeta{Kind: "Pod"},
	})
}
