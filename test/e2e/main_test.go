package e2e

import (
	"testing"

	"github.com/operator-framework/operator-sdk/pkg/test"
	"github.com/tektoncd/operator/pkg/apis"
	op "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/test/testgroups"
	_ "github.com/tektoncd/plumbing/scripts"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMain(m *testing.M) {
	test.MainEntry(m)
}

func TestPipelineOperator(t *testing.T) {
	initTestingFramework(t)

	// Run test groups (test each CRDs)
	t.Run("pipeline-crd", testgroups.ClusterCRD)
	t.Run("addon-crd", testgroups.AddonCRD)
}

func initTestingFramework(t *testing.T) {
	apiVersion := "operator.tekton.dev/v1alpha1"
	kind := "TektonPipeline"

	tektonPipelineList := &op.TektonPipelineList{
		TypeMeta: metav1.TypeMeta{
			Kind:       kind,
			APIVersion: apiVersion,
		},
	}

	if err := test.AddToFrameworkScheme(apis.AddToScheme, tektonPipelineList); err != nil {
		t.Fatalf("failed to add '%s %s': %v", apiVersion, kind, err)
	}
}
