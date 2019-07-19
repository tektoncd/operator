package e2e

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/tektoncd/operator/pkg/apis"
	op "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/test/testgroups"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMain(m *testing.M) {
	test.MainEntry(m)
}

func TestPipelineOperator(t *testing.T) {
	initTestingFramework(t)

	// Run test groups (test each CRDs)
	t.Run("config-crd", testgroups.ClusterCRD)
}

func initTestingFramework(t *testing.T) {
	apiVersion := "operator.tekton.dev/v1alpha1"
	kind := "Config"

	configList := &op.ConfigList{
		TypeMeta: metav1.TypeMeta{
			Kind:       kind,
			APIVersion: apiVersion,
		},
	}

	if err := test.AddToFrameworkScheme(apis.AddToScheme, configList); err != nil {
		t.Fatalf("failed to add '%s %s': %v", apiVersion, kind, err)
	}
}
