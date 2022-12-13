package webhook

import (
	"path/filepath"
	"testing"

	mf "github.com/manifestival/manifestival"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	KoEnvKey = "KO_DATA_PATH"
)

func TestCreateWebhookResources(t *testing.T) {
	t.Run("transform manifest when pod namespace env is set", func(t *testing.T) {
		namespace_new := "not-the-default-value"
		testdataPath := filepath.Join("testdata", "validating-defaulting-webhook")
		m, err := mf.ManifestFrom(mf.Path(testdataPath))
		assert.NilError(t, err)
		t.Setenv(POD_NAMESPACE_ENV_KEY, namespace_new)
		mOut, err := manifestTransform(&m)
		assert.NilError(t, err)
		for _, res := range mOut.Resources() {
			assertServiceNamespace(t, &res, namespace_new)
		}

	})
	t.Run("return err when pod namespace env is not set", func(t *testing.T) {
		testdataPath := filepath.Join("testdata", "validating-defaulting-webhook")
		m, err := mf.ManifestFrom(mf.Path(testdataPath))
		assert.NilError(t, err)
		_, err = manifestTransform(&m)
		assert.Error(t, err, ERR_NAMESPACE_ENV_NOT_SET.Error())
	})
}

func assertServiceNamespace(t *testing.T, u *unstructured.Unstructured, ns string) {
	t.Helper()
	hooks, _, _ := unstructured.NestedFieldNoCopy(u.Object, "webhooks")
	for _, hook := range hooks.([]interface{}) {
		srv, found, err := unstructured.NestedFieldNoCopy(hook.(map[string]interface{}), "clientConfig", "service")
		if err != nil {
			assert.NilError(t, err)
		}
		if found {
			m := srv.(map[string]interface{})
			got := m["namespace"]
			if got != ns {
				t.Errorf("expected namespace %q, got %q", ns, got)
			}
		}
	}

}
