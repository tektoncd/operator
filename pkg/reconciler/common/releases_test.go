/*
Copyright 2019 The Tekton Authors
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

package common

//"fmt"
//"os"
//"testing"
//
//mf "github.com/manifestival/manifestival"
//"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
//util "github.com/tektoncd/operator/pkg/reconciler/common/testing"

const (
	VERSION              = "0.16.0"
	SERVING_CORE         = "testdata/kodata/knative-serving/" + VERSION + "/serving-core.yaml"
	SERVING_HPA          = "testdata/kodata/knative-serving/" + VERSION + "/serving-hpa.yaml"
	EVENTING_CORE        = "testdata/kodata/knative-eventing/" + VERSION + "/eventing-core.yaml"
	IN_MEMORY_CHANNEL    = "testdata/kodata/knative-eventing/" + VERSION + "/in-memory-channel.yaml"
	SERVING_VERSION_CORE = "testdata/kodata/knative-serving/${VERSION}/serving-core.yaml"
	SERVING_VERSION_HPA  = "testdata/kodata/knative-serving/${VERSION}/serving-hpa.yaml"
)

//func TestRetrieveManifestPath(t *testing.T) {
//	koPath := "testdata/kodata"
//	os.Setenv(KoEnvKey, koPath)
//	defer os.Unsetenv(KoEnvKey)
//
//	tests := []struct {
//		component v1alpha1.TektonComponent
//		version   string
//		name      string
//		expected  string
//	}{{
//		name:      "Valid Knative Serving Version",
//		component: &v1alpha1.TektonPipeline{},
//		version:   "0.14.0",
//		expected:  koPath + "/knative-serving/0.14.0",
//	}, {
//		name: "Valid Knative Serving URLs",
//		component: &v1alpha1.KnativeServing{
//			Spec: v1alpha1.KnativeServingSpec{
//				CommonSpec: v1alpha1.CommonSpec{
//					Manifests: []v1alpha1.Manifest{{
//						Url: SERVING_CORE,
//					}, {
//						Url: SERVING_HPA,
//					}},
//				},
//			},
//		},
//		version:  VERSION,
//		expected: SERVING_CORE + "," + SERVING_HPA,
//	}, {
//		name: "Valid Knative Eventing URLs",
//		component: &v1alpha1.KnativeEventing{
//			Spec: v1alpha1.KnativeEventingSpec{
//				CommonSpec: v1alpha1.CommonSpec{
//					Manifests: []v1alpha1.Manifest{{
//						Url: EVENTING_CORE,
//					}, {
//						Url: IN_MEMORY_CHANNEL,
//					}},
//				},
//			},
//		},
//		version:  VERSION,
//		expected: EVENTING_CORE + "," + IN_MEMORY_CHANNEL,
//	}, {
//		name: "Valid Knative Serving URLs with the version parameter",
//		component: &v1alpha1.KnativeServing{
//			Spec: v1alpha1.KnativeServingSpec{
//				CommonSpec: v1alpha1.CommonSpec{
//					Manifests: []v1alpha1.Manifest{{
//						Url: SERVING_VERSION_CORE,
//					}, {
//						Url: SERVING_VERSION_HPA,
//					}},
//				},
//			},
//		},
//		version:  VERSION,
//		expected: SERVING_CORE + "," + SERVING_HPA,
//	}}
//
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			manifestPath := manifestPath(test.version, test.component)
//			util.AssertEqual(t, manifestPath, test.expected)
//			manifest, err := mf.NewManifest(manifestPath)
//			util.AssertEqual(t, err, nil)
//			util.AssertEqual(t, len(manifest.Resources()) > 0, true)
//		})
//	}
//
//	invalidPathTests := []struct {
//		component v1alpha1.TektonComponent
//		version   string
//		name      string
//		expected  string
//	}{{
//		name:      "Invalid Knative Serving Version",
//		component: &v1alpha1.TektonPipeline{},
//		version:   "invalid-version",
//		expected:  "",
//	}}
//
//	for _, test := range invalidPathTests {
//		t.Run(test.name, func(t *testing.T) {
//			manifestPath := manifestPath(test.version, test.component)
//			util.AssertEqual(t, manifestPath, test.expected)
//			manifest, err := mf.NewManifest(manifestPath)
//			util.AssertEqual(t, err != nil, true)
//			util.AssertEqual(t, len(manifest.Resources()) == 0, true)
//		})
//	}
//}
//
//func TestGetLatestRelease(t *testing.T) {
//	koPath := "testdata/kodata"
//
//	tests := []struct {
//		name      string
//		component v1alpha1.TektonComponent
//		expected  string
//	}{{
//		name:      "serving",
//		component: &v1alpha1.TektonPipeline{},
//		expected:  VERSION,
//	}}
//
//	os.Setenv(KoEnvKey, koPath)
//	defer os.Unsetenv(KoEnvKey)
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			version := latestRelease(test.component)
//			util.AssertEqual(t, version, test.expected)
//		})
//	}
//}
//
//func TestListReleases(t *testing.T) {
//	koPath := "testdata/kodata"
//
//	tests := []struct {
//		name      string
//		component v1alpha1.TektonComponent
//		expected  []string
//	}{{
//		name:      "knative-serving",
//		component: &v1alpha1.KnativeServing{},
//		expected:  []string{"0.16.0", "0.15.0", "0.14.0"},
//	}}
//
//	os.Setenv(KoEnvKey, koPath)
//	defer os.Unsetenv(KoEnvKey)
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			version, err := allReleases(test.component)
//			util.AssertEqual(t, err, nil)
//			util.AssertDeepEqual(t, version, test.expected)
//		})
//	}
//}
//
//func TestIsUpDowngradeEligible(t *testing.T) {
//	koPath := "testdata/kodata"
//	tests := []struct {
//		name      string
//		component v1alpha1.TektonComponent
//		expected  bool
//	}{{
//		name: "knative-serving without status.version",
//		component: &v1alpha1.KnativeServing{
//			Spec: v1alpha1.KnativeServingSpec{
//				CommonSpec: v1alpha1.CommonSpec{
//					Version: "0.14.2",
//				},
//			},
//		},
//		expected: true,
//	}, {
//		name: "knative-serving upgrading one minor version",
//		component: &v1alpha1.KnativeServing{
//			Spec: v1alpha1.KnativeServingSpec{
//				CommonSpec: v1alpha1.CommonSpec{
//					Version: "0.14.2",
//				},
//			},
//			Status: v1alpha1.KnativeServingStatus{
//				Version: "0.13.0",
//			},
//		},
//		expected: true,
//	}, {
//		name: "knative-serving upgrading across multiple minor versions",
//		component: &v1alpha1.KnativeServing{
//			Spec: v1alpha1.KnativeServingSpec{
//				CommonSpec: v1alpha1.CommonSpec{
//					Version: "0.15.0",
//				},
//			},
//			Status: v1alpha1.KnativeServingStatus{
//				Version: "0.13.0",
//			},
//		},
//		expected: false,
//	}, {
//		name: "knative-serving upgrading to the latest version across multiple minor versions",
//		component: &v1alpha1.KnativeServing{
//			Status: v1alpha1.KnativeServingStatus{
//				Version: "0.13.0",
//			},
//		},
//		// The latest version is 0.15.0
//		expected: false,
//	}, {
//		name: "knative-serving upgrading to the latest version",
//		component: &v1alpha1.KnativeServing{
//			Status: v1alpha1.KnativeServingStatus{
//				Version: "0.15.0",
//			},
//		},
//		// The latest version is 0.16.0
//		expected: true,
//	}, {
//		name:      "knative-serving with latest version and empty status.version",
//		component: &v1alpha1.KnativeServing{},
//		expected:  true,
//	}, {
//		name: "knative-serving with the same status.version and spec.version",
//		component: &v1alpha1.KnativeServing{
//			Spec: v1alpha1.KnativeServingSpec{
//				CommonSpec: v1alpha1.CommonSpec{
//					Version: "0.15.0",
//				},
//			},
//			Status: v1alpha1.KnativeServingStatus{
//				Version: "0.15.0",
//			},
//		},
//		expected: true,
//	}}
//
//	os.Setenv(KoEnvKey, koPath)
//	defer os.Unsetenv(KoEnvKey)
//
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			result := IsUpDowngradeEligible(test.component)
//			util.AssertEqual(t, result, test.expected)
//		})
//	}
//}
//
//func TestTargetManifest(t *testing.T) {
//	tests := []struct {
//		name          string
//		component     v1alpha1.TektonComponent
//		expectedError error
//	}{{
//		name: "knative-serving with spec.manifests matched",
//		component: &v1alpha1.KnativeServing{
//			Spec: v1alpha1.KnativeServingSpec{
//				CommonSpec: v1alpha1.CommonSpec{
//					Version: "0.16.0",
//					Manifests: []v1alpha1.Manifest{{
//						Url: "testdata/kodata/knative-serving/${VERSION}/serving-core.yaml",
//					}, {
//						Url: "testdata/kodata/knative-serving/${VERSION}/serving-hpa.yaml",
//					}},
//				},
//			},
//		},
//		expectedError: nil,
//	}, {
//		name: "knative-serving with spec.manifests unmatched",
//		component: &v1alpha1.KnativeServing{
//			Spec: v1alpha1.KnativeServingSpec{
//				CommonSpec: v1alpha1.CommonSpec{
//					Version: "0.16.0",
//					Manifests: []v1alpha1.Manifest{{
//						Url: "testdata/invalid_kodata/knative-serving/${VERSION}_unmatched_version/serving-core.yaml",
//					}, {
//						Url: "testdata/invalid_kodata/knative-serving/${VERSION}_unmatched_version/serving-hpa.yaml",
//					}},
//				},
//			},
//		},
//		expectedError: fmt.Errorf("The version of the manifests %s does not match the target "+
//			"version of the operator CR %s. The resource name is %s.", "v0.16.2", "v0.16.0", "knative-serving"),
//	}, {
//		name: "knative-serving with spec.manifests matched but no spec.version",
//		component: &v1alpha1.KnativeServing{
//			Spec: v1alpha1.KnativeServingSpec{
//				CommonSpec: v1alpha1.CommonSpec{
//					Manifests: []v1alpha1.Manifest{{
//						Url: "testdata/kodata/knative-serving/0.16.0/serving-core.yaml",
//					}, {
//						Url: "testdata/kodata/knative-serving/0.16.0/serving-hpa.yaml",
//					}},
//				},
//			},
//		},
//		expectedError: nil,
//	}}
//
//	koPath := "testdata/kodata"
//	os.Setenv(KoEnvKey, koPath)
//	defer os.Unsetenv(KoEnvKey)
//
//	for _, test := range tests {
//		t.Run(test.name, func(t *testing.T) {
//			_, err := TargetManifest(test.component)
//			if err != test.expectedError {
//				util.AssertEqual(t, err.Error(), test.expectedError.Error())
//			}
//		})
//	}
//}
