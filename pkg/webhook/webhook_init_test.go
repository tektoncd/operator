package webhook

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	k8sfake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"gotest.tools/v3/assert"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
		assert.Error(t, err, ErrNamespaceEnvNotSet.Error())
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

func TestDeleteExistingInstallerSets(t *testing.T) {
	tests := []struct {
		name                      string
		podName                   string
		includeUniqueIdentifier   bool
		installerSets             []v1alpha1.TektonInstallerSet
		expectedInstallerSetCount int
		expectedError             string
	}{
		{
			name:                    "remove-deprecated-webhook-installersets",
			podName:                 "",
			includeUniqueIdentifier: false,
			installerSets: []v1alpha1.TektonInstallerSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "set1",
						Labels: map[string]string{
							DEPRECATED_WEBHOOK_INSTALLERSET_LABEL: "",
						},
					},
				},
			},
			expectedInstallerSetCount: 0,
		},
		{
			name:                    "without-pod-name-env-set",
			podName:                 "",
			includeUniqueIdentifier: true,
			installerSets: []v1alpha1.TektonInstallerSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "set1",
						Labels: map[string]string{
							v1alpha1.CreatedByKey:     labelCreatedByValue,
							v1alpha1.InstallerSetType: labelInstallerSetTypeValue,
							WEBHOOK_UNIQUE_LABEL:      "webhook-foo",
						},
					},
				},
			},
			expectedInstallerSetCount: 1,
			expectedError:             fmt.Sprintf("pod name environment variable[%s] details are not set", POD_NAME_ENV_KEY),
		},
		{
			name:                      "delete-on-empty-set-with-pod-name",
			podName:                   "webhook-foo",
			includeUniqueIdentifier:   true,
			installerSets:             []v1alpha1.TektonInstallerSet{},
			expectedInstallerSetCount: 0,
		},
		{
			name:                      "delete-on-empty-set-without-pod-name",
			podName:                   "",
			includeUniqueIdentifier:   false,
			installerSets:             []v1alpha1.TektonInstallerSet{},
			expectedInstallerSetCount: 0,
		},
		{
			name:                    "with-pod-name-env-set-and-mismatch",
			podName:                 "webhook-name-foo",
			includeUniqueIdentifier: true,
			installerSets: []v1alpha1.TektonInstallerSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "set1",
						Labels: map[string]string{
							v1alpha1.CreatedByKey:     labelCreatedByValue,
							v1alpha1.InstallerSetType: labelInstallerSetTypeValue,
							WEBHOOK_UNIQUE_LABEL:      "webhook-foo",
						},
					},
				},
			},
			expectedInstallerSetCount: 1,
		},
		{
			name:                    "with-pod-name-env-set-and-match",
			podName:                 "webhook-name-foo",
			includeUniqueIdentifier: true,
			installerSets: []v1alpha1.TektonInstallerSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "set1",
						Labels: map[string]string{
							v1alpha1.CreatedByKey:     labelCreatedByValue,
							v1alpha1.InstallerSetType: labelInstallerSetTypeValue,
							WEBHOOK_UNIQUE_LABEL:      "webhook-name-foo",
						},
					},
				},
			},
			expectedInstallerSetCount: 0,
		},
		{
			name:                    "mismatch-labels",
			podName:                 "",
			includeUniqueIdentifier: false,
			installerSets: []v1alpha1.TektonInstallerSet{
				{
					ObjectMeta: v1.ObjectMeta{
						Name: "set1",
						Labels: map[string]string{
							"foo": "bar",
						},
					},
				},
			},
			expectedInstallerSetCount: 1,
		},
	}

	ctx := context.Background()
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			// get fake client
			k8sClient := k8sfake.NewSimpleClientset()

			if test.podName != "" {
				t.Setenv(POD_NAME_ENV_KEY, test.podName)
			}

			// update existing installersets
			for _, installerSet := range test.installerSets {
				_, err := k8sClient.OperatorV1alpha1().TektonInstallerSets().Create(ctx, &installerSet, v1.CreateOptions{})
				assert.NilError(t, err)
			}

			// run delete installersets
			err := deleteExistingInstallerSets(ctx, k8sClient, test.includeUniqueIdentifier)
			if test.expectedError != "" {
				assert.Error(t, err, test.expectedError)
			} else {
				assert.NilError(t, err)
			}

			// verify the expected count
			installerSetsList, err := k8sClient.OperatorV1alpha1().TektonInstallerSets().List(ctx, v1.ListOptions{})
			assert.NilError(t, err)
			assert.Equal(t, test.expectedInstallerSetCount, len(installerSetsList.Items))

		})
	}

}
