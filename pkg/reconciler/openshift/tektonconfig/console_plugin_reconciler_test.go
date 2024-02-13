/*
Copyright 2024 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tektonconfig

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	k8sTesting "k8s.io/client-go/testing"
	"knative.dev/pkg/logging"
)

// reactor required for the GenerateName field to work when using the fake client
func generateNameReactor(action k8sTesting.Action) (bool, apimachineryRuntime.Object, error) {
	resource := action.(k8sTesting.CreateAction).GetObject()
	meta, ok := resource.(metav1.Object)
	if !ok {
		return false, resource, nil
	}

	if meta.GetName() == "" && meta.GetGenerateName() != "" {
		meta.SetName(common.SimpleNameGenerator.RestrictLengthWithRandomSuffix(meta.GetGenerateName()))
	}
	return false, resource, nil
}

func TestPostReconcileManifest(t *testing.T) {
	defaultConsolePluginImage := "ghcr.io/openshift-pipelines/console-plugin:main"

	tests := []struct {
		name               string
		consolePluginImage string
		operatorVersion    string
		targetNamespace    string
	}{
		{
			name:            "test-without-console-plugin-image",
			operatorVersion: "1.14.0",
			targetNamespace: "foo",
		},
		{
			name:               "test-with-console-plugin-image",
			consolePluginImage: "custom-image:tag1",
			operatorVersion:    "0.70.0",
			targetNamespace:    "bar",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {

			ctx := context.TODO()
			operatorFakeClientSet := fake.NewSimpleClientset()

			// add reactor to update generateName
			operatorFakeClientSet.PrependReactor("create", "*", generateNameReactor)

			// TEST: verifies required values in generated manifests (InstallerSet)
			verifyManifestFunc := func(expectedImage, expectedOperatorVersion string) {
				// verify installersets availability
				installerSetList, err := operatorFakeClientSet.OperatorV1alpha1().TektonInstallerSets().List(
					ctx,
					metav1.ListOptions{LabelSelector: fmt.Sprintf("operator.tekton.dev/created-by=%s", consolePluginReconcileLabelCreatedByValue)},
				)
				require.NoError(t, err)

				require.Equal(t, 1, len(installerSetList.Items))
				installerSet := installerSetList.Items[0]

				// verify operator version label
				operatorVersion := installerSet.GetLabels()[v1alpha1.ReleaseVersionKey]
				require.Equal(t, expectedOperatorVersion, operatorVersion)

				// get installerset and verify transform values
				for _, u := range installerSet.Spec.Manifests {
					// verify targetNamespace
					require.Equal(t, test.targetNamespace, u.GetNamespace())

					switch u.GetKind() {
					case "Deployment":
						deployment := &appsv1.Deployment{}
						err := apimachineryRuntime.DefaultUnstructuredConverter.FromUnstructured(u.Object, deployment)
						require.NoError(t, err)
						require.Equal(t, "pipelines-console-plugin", deployment.GetName())
						container := deployment.Spec.Template.Spec.Containers[0]
						require.Equal(t, expectedImage, container.Image)

					case "ConsolePlugin":
						actualNamespace, found, err := unstructured.NestedString(u.Object, "spec", "backend", "service", "namespace")
						require.NoError(t, err)
						require.True(t, found)
						require.Equal(t, test.targetNamespace, actualNamespace)

					}
				}
			}

			// reconciler reference
			postReconcile := &consolePluginReconciler{
				logger:                 logging.FromContext(ctx).Named("post-reconcile-manifest-test"),
				operatorClientSet:      operatorFakeClientSet,
				syncOnce:               sync.Once{},
				resourcesYamlDirectory: "./testdata/postreconcile_manifest",
				operatorVersion:        test.operatorVersion,
			}

			// tekton config CR
			tektonConfigCR := &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.ConfigResourceName,
				},
				Spec: v1alpha1.TektonConfigSpec{
					CommonSpec: v1alpha1.CommonSpec{
						TargetNamespace: test.targetNamespace,
					},
				},
			}

			// console plugin image
			consolePluginImage := defaultConsolePluginImage
			// update image env variable
			if test.consolePluginImage != "" {
				t.Setenv("IMAGE_PIPELINES_CONSOLE_PLUGIN", test.consolePluginImage)
				consolePluginImage = test.consolePluginImage
			}
			// TEST: image name
			err := postReconcile.reconcile(ctx, tektonConfigCR) // perform reconcile
			require.NoError(t, err)
			verifyManifestFunc(consolePluginImage, test.operatorVersion) // verify manifests

			// TEST: operator version change
			// update operator version in installerSet and reconcile
			postReconcile.operatorVersion = "foo"
			err = postReconcile.reconcile(ctx, tektonConfigCR) // perform reconcile
			require.NoError(t, err)
			verifyManifestFunc(consolePluginImage, "foo") // verify
			postReconcile.operatorVersion = test.operatorVersion

			// TEST: removal of extra installerSet
			// add another tekton config manifest post reconcile installerset and reconcile
			newInstallerSet := &v1alpha1.TektonInstallerSet{
				ObjectMeta: metav1.ObjectMeta{
					GenerateName: "another-tekton-config-manifest-foo-",
					Labels:       consolePluginReconcileInstallerSetLabel.MatchLabels,
				},
			}
			_, err = operatorFakeClientSet.OperatorV1alpha1().TektonInstallerSets().Create(ctx, newInstallerSet, metav1.CreateOptions{})
			require.NoError(t, err)
			err = postReconcile.reconcile(ctx, tektonConfigCR) // perform reconcile
			require.NoError(t, err)
			verifyManifestFunc(consolePluginImage, test.operatorVersion) // verify manifests

			// TEST: do not touch others installerSets
			// add another installerset(not tekton config manifest post reconcile) and reconcile
			// this installerSet should not be removed
			anotherInstallerSetName := "pipelines-foo"
			anotherInstallerSet := &v1alpha1.TektonInstallerSet{
				ObjectMeta: metav1.ObjectMeta{
					Name: anotherInstallerSetName,
				},
			}
			_, err = operatorFakeClientSet.OperatorV1alpha1().TektonInstallerSets().Create(ctx, anotherInstallerSet, metav1.CreateOptions{})
			require.NoError(t, err)
			err = postReconcile.reconcile(ctx, tektonConfigCR) // perform reconcile
			require.NoError(t, err)
			verifyManifestFunc(consolePluginImage, test.operatorVersion) // verify manifests
			installerSetList, err := operatorFakeClientSet.OperatorV1alpha1().TektonInstallerSets().List(ctx, metav1.ListOptions{})
			require.NoError(t, err)
			require.Equal(t, 2, len(installerSetList.Items))
			expectedInstallerSetFound := false
			for _, installerSet := range installerSetList.Items {
				if installerSet.GetName() == anotherInstallerSetName {
					expectedInstallerSetFound = true
					break
				}
			}
			require.True(t, expectedInstallerSetFound)
		})
	}
}
