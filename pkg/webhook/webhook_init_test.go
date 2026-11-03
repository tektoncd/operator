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
	testdataPath := filepath.Join("testdata", "validating-defaulting-webhook")

	tests := []struct {
		name                string
		namespace           string
		serviceName         string
		setNamespace        bool
		setServiceName      bool
		expectError         bool
		expectedErrMsg      string
		expectedNamespace   string
		expectedServiceName string
	}{
		{
			name:           "error when namespace env not set",
			setNamespace:   false,
			setServiceName: false,
			expectError:    true,
			expectedErrMsg: ErrNamespaceEnvNotSet.Error(),
		},
		{
			name:                "default service name when WEBHOOK_SERVICE_NAME not set",
			namespace:           "tekton-operator",
			setNamespace:        true,
			setServiceName:      false,
			expectError:         false,
			expectedNamespace:   "tekton-operator",
			expectedServiceName: DEFAULT_WEBHOOK_SERVICE,
		},
		{
			name:                "custom service name from WEBHOOK_SERVICE_NAME env",
			namespace:           "tekton-operator",
			serviceName:         "myrelease-tekton-operator-webhook",
			setNamespace:        true,
			setServiceName:      true,
			expectError:         false,
			expectedNamespace:   "tekton-operator",
			expectedServiceName: "myrelease-tekton-operator-webhook",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := mf.ManifestFrom(mf.Path(testdataPath))
			assert.NilError(t, err)

			if tt.setNamespace {
				t.Setenv(POD_NAMESPACE_ENV_KEY, tt.namespace)
			}
			if tt.setServiceName {
				t.Setenv(WEBHOOK_SERVICE_NAME_KEY, tt.serviceName)
			}

			mOut, err := manifestTransform(&m)
			if tt.expectError {
				assert.Error(t, err, tt.expectedErrMsg)
				return
			}

			assert.NilError(t, err)

			// Verify webhook configurations have correct service name and namespace
			for _, res := range mOut.Resources() {
				if res.GetKind() != "ValidatingWebhookConfiguration" && res.GetKind() != "MutatingWebhookConfiguration" {
					continue
				}

				webhooks, found, _ := unstructured.NestedSlice(res.Object, "webhooks")
				if !found {
					continue
				}

				for _, hook := range webhooks {
					hookMap := hook.(map[string]interface{})
					svcName, _, _ := unstructured.NestedString(hookMap, "clientConfig", "service", "name")
					svcNs, _, _ := unstructured.NestedString(hookMap, "clientConfig", "service", "namespace")

					assert.Equal(t, tt.expectedServiceName, svcName, "service name mismatch")
					assert.Equal(t, tt.expectedNamespace, svcNs, "namespace mismatch")
				}
			}
		})
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
