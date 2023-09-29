package cluster

import (
	"context"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type PodSelector struct {
	podClient corev1.PodInterface
	name      string
}

func (ps *PodSelector) Raw(ctx context.Context) (*v1.Pod, error) {
	return ps.podClient.Get(ctx, ps.name, metav1.GetOptions{})
}
