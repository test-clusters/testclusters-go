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

func (pl *PodList) StatusPhase(ctx context.Context, expected v1.PodPhase) error {
	list, err := pl.pods.List(ctx, pl.listOptions)
	if err != nil {
		return fmt.Errorf("could not list pods for listOptions %s: %s", pl.listOptions.String(), err.Error())
	}

	for _, pod := range list.Items {
		if pod.Status.Phase != expected {
			return fmt.Errorf("pod %s is not in expected lifecycle-pahase. expected; %s; actual: %s", pod.Name, expected, pod.Status.Phase)
		}
	}

	return nil
}

type PodSelector struct {
	pods corev1.PodInterface
}

func (s *PodSelector) ByLabels(labels string) *PodList {
	return &PodList{
		pods: s.pods,
		listOptions: metav1.ListOptions{
			LabelSelector: labels,
		},
	}
}
