package cluster

import (
	"context"
	"fmt"

	"k8s.io/client-go/rest"

	"github.com/cloudogu/k8s-apply-lib/apply"
)

type kubeApplier interface {
	Apply(yamlResource apply.YamlDocument, namespace string) error
}

// YamlApplier provides a pod with kubectl access to the cluster.
type YamlApplier struct {
	applier          kubeApplier
	defaultNamespace string
}

func NewYamlApplier(restConfig *rest.Config, fieldManager, defaultNamespace string) (*YamlApplier, error) {
	applier, _, err := apply.New(restConfig, fieldManager)
	if err != nil {
		return nil, fmt.Errorf("could not create applier: %w", err)
	}
	return &YamlApplier{applier: applier, defaultNamespace: defaultNamespace}, nil
}

// ApplyWithFile copies the yaml bytes into a file and calls `kubectl apply -f`.
func (ya *YamlApplier) ApplyWithFile(ctx context.Context, yamlBytes []byte) error {
	err := ya.applier.Apply(yamlBytes, ya.defaultNamespace)
	if err != nil {
		return err
	}
	return nil
}
