package tektonaddon

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	mfc "github.com/manifestival/controller-runtime-client"
	mf "github.com/manifestival/manifestival"
	op "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/golden"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestGeneratePipelineTemplates(t *testing.T) {
	addonLocation := filepath.Join("testdata")
	var (
		configName = "cluster"
		namespace  = "openshift-pipelines"
	)
	config := newConfig(configName, namespace)
	cl := feedConfigMock(config)

	manifest := mf.Manifest{Client: mfc.NewClient(cl)}

	err := GeneratePipelineTemplates(addonLocation, &manifest)
	assertNoEror(t, err)
	for _, m := range manifest.Resources() {
		jsonPipeline, err := m.MarshalJSON()
		assertNoEror(t, err)
		golden.Assert(t, string(jsonPipeline), strings.ReplaceAll(fmt.Sprintf("%s.golden", m.GetName()), "/", "-"))
	}
}

func assertNoEror(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("assertion failed; expected no error %v", err)
	}
}

func newConfig(name string, namespace string) *op.TektonConfig {
	return &op.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: op.TektonConfigSpec{
			Profile:    "",
			CommonSpec: op.CommonSpec{TargetNamespace: namespace},
		},
	}
}

func feedConfigMock(config *op.TektonConfig) client.Client {
	objs := []runtime.Object{config}

	// Register operator types with the runtime scheme.
	s := scheme.Scheme
	s.AddKnownTypes(op.SchemeGroupVersion, config)

	// Create a fake client to mock API calls.
	return fake.NewClientBuilder().WithScheme(s).WithRuntimeObjects(objs...).Build()
}
