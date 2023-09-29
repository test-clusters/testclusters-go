package cluster

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

type PodList struct {
	pods        corev1.PodInterface
	listOptions metav1.ListOptions
}

func (pl *PodList) Len(ctx context.Context, expected int) error {
	list, err := pl.pods.List(ctx, pl.listOptions)
	if err != nil {
		return fmt.Errorf("could not list pods for listOptions %s: %w", pl.listOptions.String(), err)
	}

	itemsLen := len(list.Items)
	if itemsLen != expected {
		return fmt.Errorf("did not find expected number of pods: expected: %d; actual: %d", expected, itemsLen)
	}

	return nil
}

// Raw queries the kubernetes API and returns the pod list as plain kubernetes API objects.
func (pl *PodList) Raw(ctx context.Context) (*v1.PodList, error) {
	return pl.pods.List(ctx, pl.listOptions)
}

type PodSelector struct {
	pods        corev1.PodInterface
	listOptions metav1.ListOptions
}

func (s *PodSelector) ByLabels(labels string) *PodSelector {
	ps := &PodSelector{
		pods:        s.pods,
		listOptions: s.listOptions,
	}
	ps.listOptions.LabelSelector = labels
	return ps
}

func (s *PodSelector) ByFieldSelector(fieldSelector string) *PodSelector {
	ps := &PodSelector{
		pods:        s.pods,
		listOptions: s.listOptions,
	}
	ps.listOptions.FieldSelector = fieldSelector
	return ps
}

func (s *PodSelector) List() *PodList {
	return &PodList{
		pods:        s.pods,
		listOptions: s.listOptions,
	}
}
