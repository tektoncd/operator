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

package v1alpha1

import (
	"testing"

	"go.uber.org/zap"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func TestSetPACControllerDefaultSettings(t *testing.T) {
	opacCR := &OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: OpenShiftPipelinesAsCodeSpec{
			PACSettings: PACSettings{
				Settings: map[string]string{},
			},
		},
	}

	opacCR.Spec.PACSettings.setPACDefaults(zap.NewNop().Sugar())

	expectedSettings := map[string]string{
		"application-name":                           "Pipelines as Code CI",
		"auto-configure-new-github-repo":             "false",
		"auto-configure-repo-namespace-template":     "",
		"auto-configure-repo-repository-template":    "",
		"bitbucket-cloud-additional-source-ip":       "",
		"bitbucket-cloud-check-source-ip":            "true",
		"custom-console-name":                        "",
		"custom-console-url":                         "",
		"custom-console-url-namespace":               "",
		"custom-console-url-pr-details":              "",
		"custom-console-url-pr-tasklog":              "",
		"default-max-keep-runs":                      "0",
		"enable-cancel-in-progress-on-pull-requests": "false",
		"enable-cancel-in-progress-on-push":          "false",
		"error-detection-from-container-logs":        "true",
		"error-detection-max-number-of-lines":        "50",
		"error-detection-simple-regexp":              "^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+)?([ ]*)?(?P<error>.*)",
		"error-log-snippet":                          "true",
		"error-log-snippet-number-of-lines":          "3",
		"hub-catalog-type":                           "artifacthub",
		"hub-url":                                    "https://artifacthub.io",
		"max-keep-run-upper-limit":                   "0",
		"remember-ok-to-test":                        "false",
		"skip-push-event-for-pr-commits":             "true",
		"remote-tasks":                               "true",
		"secret-auto-create":                         "true",
		"secret-github-app-scope-extra-repos":        "",
		"secret-github-app-token-scoped":             "true",
		"tekton-dashboard-url":                       "",
	}

	assert.DeepEqual(t, opacCR.Spec.PACSettings.Settings, expectedSettings)
}

func TestSetPACControllerLimitedSettings(t *testing.T) {
	opacCR := &OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: OpenShiftPipelinesAsCodeSpec{
			PACSettings: PACSettings{
				Settings: map[string]string{
					"application-name":                       "Pipelines as Code CI test name",
					"auto-configure-new-github-repo":         "false",
					"auto-configure-repo-namespace-template": "",
					"bitbucket-cloud-additional-source-ip":   "",
					"error-detection-from-container-logs":    "true",
					"error-detection-max-number-of-lines":    "100",
					"remote-tasks":                           "",
				},
			},
		},
	}

	opacCR.Spec.PACSettings.setPACDefaults(zap.NewNop().Sugar())

	expectedSettings := map[string]string{
		"application-name":                           "Pipelines as Code CI test name",
		"auto-configure-new-github-repo":             "false",
		"auto-configure-repo-namespace-template":     "",
		"auto-configure-repo-repository-template":    "",
		"bitbucket-cloud-additional-source-ip":       "",
		"bitbucket-cloud-check-source-ip":            "true",
		"custom-console-name":                        "",
		"custom-console-url":                         "",
		"custom-console-url-namespace":               "",
		"custom-console-url-pr-details":              "",
		"custom-console-url-pr-tasklog":              "",
		"default-max-keep-runs":                      "0",
		"enable-cancel-in-progress-on-pull-requests": "false",
		"enable-cancel-in-progress-on-push":          "false",
		"error-detection-from-container-logs":        "true",
		"error-detection-max-number-of-lines":        "100",
		"error-detection-simple-regexp":              "^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+)?([ ]*)?(?P<error>.*)",
		"error-log-snippet":                          "true",
		"error-log-snippet-number-of-lines":          "3",
		"hub-catalog-type":                           "artifacthub",
		"hub-url":                                    "https://artifacthub.io",
		"max-keep-run-upper-limit":                   "0",
		"remember-ok-to-test":                        "false",
		"skip-push-event-for-pr-commits":             "true",
		"remote-tasks":                               "true",
		"secret-auto-create":                         "true",
		"secret-github-app-scope-extra-repos":        "",
		"secret-github-app-token-scoped":             "true",
		"tekton-dashboard-url":                       "",
	}

	assert.DeepEqual(t, opacCR.Spec.PACSettings.Settings, expectedSettings)
}

func TestSetPACControllerDefaultSettingsWithMultipleCatalogs(t *testing.T) {
	opacCR := &OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: OpenShiftPipelinesAsCodeSpec{
			PACSettings: PACSettings{
				Settings: map[string]string{
					"catalog-1-id":   "anotherhub",
					"catalog-1-name": "tekton",
					"catalog-1-url":  "https://api.other.com/v1",
					"catalog-5-id":   "anotherhub5",
					"catalog-5-name": "tekton1",
					"catalog-5-url":  "https://api.other.com/v2",
				},
			},
		},
	}

	opacCR.Spec.PACSettings.setPACDefaults(zap.NewNop().Sugar())

	expectedSettings := map[string]string{
		"application-name":                           "Pipelines as Code CI",
		"auto-configure-new-github-repo":             "false",
		"auto-configure-repo-namespace-template":     "",
		"auto-configure-repo-repository-template":    "",
		"bitbucket-cloud-additional-source-ip":       "",
		"bitbucket-cloud-check-source-ip":            "true",
		"catalog-1-id":                               "anotherhub",
		"catalog-1-name":                             "tekton",
		"catalog-1-url":                              "https://api.other.com/v1",
		"catalog-5-id":                               "anotherhub5",
		"catalog-5-name":                             "tekton1",
		"catalog-5-url":                              "https://api.other.com/v2",
		"custom-console-name":                        "",
		"custom-console-url":                         "",
		"custom-console-url-namespace":               "",
		"custom-console-url-pr-details":              "",
		"custom-console-url-pr-tasklog":              "",
		"default-max-keep-runs":                      "0",
		"enable-cancel-in-progress-on-pull-requests": "false",
		"enable-cancel-in-progress-on-push":          "false",
		"error-detection-from-container-logs":        "true",
		"error-detection-max-number-of-lines":        "50",
		"error-detection-simple-regexp":              "^(?P<filename>[^:]*):(?P<line>[0-9]+):(?P<column>[0-9]+)?([ ]*)?(?P<error>.*)",
		"error-log-snippet":                          "true",
		"error-log-snippet-number-of-lines":          "3",
		"hub-catalog-type":                           "artifacthub",
		"hub-url":                                    "https://artifacthub.io",
		"max-keep-run-upper-limit":                   "0",
		"remember-ok-to-test":                        "false",
		"skip-push-event-for-pr-commits":             "true",
		"remote-tasks":                               "true",
		"secret-auto-create":                         "true",
		"secret-github-app-scope-extra-repos":        "",
		"secret-github-app-token-scoped":             "true",
		"tekton-dashboard-url":                       "",
	}

	assert.DeepEqual(t, opacCR.Spec.PACSettings.Settings, expectedSettings)
}

func TestSetAdditionalPACControllerDefault(t *testing.T) {
	opacCR := &OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: OpenShiftPipelinesAsCodeSpec{
			PACSettings: PACSettings{
				Settings: map[string]string{},
				AdditionalPACControllers: map[string]AdditionalPACControllerConfig{
					"test": {},
				},
			},
		},
	}

	opacCR.Spec.PACSettings.setPACDefaults(zap.NewNop().Sugar())

	assert.Equal(t, true, *opacCR.Spec.PACSettings.AdditionalPACControllers["test"].Enable)
	assert.Equal(t, "test-pipelines-as-code-configmap", opacCR.Spec.PACSettings.AdditionalPACControllers["test"].ConfigMapName)
	assert.Equal(t, "test-pipelines-as-code-secret", opacCR.Spec.PACSettings.AdditionalPACControllers["test"].SecretName)
}

func TestSetAdditionalPACControllerDefaultHavingAdditionalPACController(t *testing.T) {
	opacCR := &OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: OpenShiftPipelinesAsCodeSpec{
			PACSettings: PACSettings{
				Settings: map[string]string{},
				AdditionalPACControllers: map[string]AdditionalPACControllerConfig{
					"test": {
						Enable:        ptr.Bool(false),
						ConfigMapName: "test-configmap",
						SecretName:    "test-secret",
						Settings: map[string]string{
							"application-name":    "Additional PACController CI",
							"custom-console-name": "custom",
							"custom-console-url":  "https://custom.com",
						},
					},
				},
			},
		},
	}

	opacCR.Spec.PACSettings.setPACDefaults(zap.NewNop().Sugar())

	assert.Equal(t, false, *opacCR.Spec.PACSettings.AdditionalPACControllers["test"].Enable)
	assert.Equal(t, "Additional PACController CI", opacCR.Spec.PACSettings.AdditionalPACControllers["test"].Settings["application-name"])
	assert.Equal(t, "custom", opacCR.Spec.PACSettings.AdditionalPACControllers["test"].Settings["custom-console-name"])
	assert.Equal(t, "https://custom.com", opacCR.Spec.PACSettings.AdditionalPACControllers["test"].Settings["custom-console-url"])
}
