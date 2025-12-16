/*
Copyright 2023 The Tekton Authors

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

package upgrade

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonpruner"
	v1 "k8s.io/api/core/v1"
	k8sFake "k8s.io/client-go/kubernetes/fake"

	"github.com/stretchr/testify/assert"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorFake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/ptr"
)

func TestResetTektonConfigConditions(t *testing.T) {
	ctx := context.TODO()
	operatorClient := operatorFake.NewSimpleClientset()
	logger := logging.FromContext(ctx).Named("unit-test")

	// there is no tektonConfig CR, returns no error
	err := resetTektonConfigConditions(ctx, logger, nil, operatorClient, nil)
	assert.NoError(t, err)

	// create tekconConfig CR with initial conditions
	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Status: v1alpha1.TektonConfigStatus{
			Status: duckv1.Status{
				Conditions: duckv1.Conditions{{
					Type:   v1alpha1.AllDeploymentsReady,
					Status: "notReady",
				}},
			},
		},
	}
	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Create(ctx, tc, metav1.CreateOptions{})
	assert.NoError(t, err)

	// resets the conditions
	err = resetTektonConfigConditions(ctx, logger, nil, operatorClient, nil)
	assert.NoError(t, err)

	// verify the conditions field is empty
	_tc, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Empty(t, _tc.Status.Conditions)
}

func TestUpgradePipelineProperties(t *testing.T) {
	tests := []struct {
		name     string
		tc       *v1alpha1.TektonConfig
		expected bool
	}{
		{
			name: "with explicit false enable step actions",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.ConfigResourceName,
				},
				Spec: v1alpha1.TektonConfigSpec{
					Pipeline: v1alpha1.Pipeline{
						PipelineProperties: v1alpha1.PipelineProperties{
							EnableStepActions: ptr.Bool(false),
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "with default pipeline properties",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.ConfigResourceName,
				},
			},
			expected: true,
		},
	}

	ctx := context.TODO()
	logger := logging.FromContext(ctx).Named("unit-test")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operatorClient := operatorFake.NewSimpleClientset()

			// test when no tektonConfig CR exists
			err := upgradePipelineProperties(ctx, logger, nil, operatorClient, nil)
			assert.NoError(t, err)

			// create tektonConfig CR
			if tt.tc != nil {
				_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Create(ctx, tt.tc, metav1.CreateOptions{})
				assert.NoError(t, err)
			}

			// update enable-step-actions to true
			err = upgradePipelineProperties(ctx, logger, nil, operatorClient, nil)
			assert.NoError(t, err)

			// verify the pipeline property enable-step-actions is set to true
			tcData, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
			assert.NoError(t, err)
			assert.Equal(t, *tcData.Spec.Pipeline.EnableStepActions, tt.expected)
		})
	}
}

func TestPreUpgradeTektonPruner(t *testing.T) {
	ctx := context.TODO()
	operatorClient := operatorFake.NewSimpleClientset()
	k8sClient := k8sFake.NewSimpleClientset()
	logger := logging.FromContext(ctx).Named("unit-test")

	// there is no tektonConfig CR available, returns error
	err := preUpgradeTektonPruner(ctx, logger, k8sClient, operatorClient, nil)
	assert.Error(t, err)

	// create tekconConfig CR
	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "foo",
			},
			TektonPruner: v1alpha1.Pruner{
				Disabled: ptr.Bool(false),
			},
		},
	}
	_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Create(ctx, tc, metav1.CreateOptions{})
	assert.NoError(t, err)

	// there is no tekton-config configMap, return no error
	err = preUpgradeTektonPruner(ctx, logger, k8sClient, operatorClient, nil)
	assert.NoError(t, err)

	// verify chains existing field, should not be empty
	tc, err = operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, false, *tc.Spec.TektonPruner.Disabled)

	// create a config map with values
	config := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      tektonpruner.PrunerConfigMapName,
			Namespace: tc.Spec.GetTargetNamespace(),
		},
		Data: map[string]string{
			"global-config": "enforcedConfigLevel: global\nttlSecondsAfterFinished: 88\nsuccessfulHistoryLimit: 400\nfailedHistoryLimit: 10\nhistoryLimit: 10\nnamespaces: {}\n",
		},
	}
	_, err = k8sClient.CoreV1().ConfigMaps(tc.Spec.GetTargetNamespace()).Create(ctx, config, metav1.CreateOptions{})
	assert.NoError(t, err)

	cm, _ := k8sClient.CoreV1().ConfigMaps(tc.Spec.GetTargetNamespace()).Get(ctx, tektonpruner.PrunerConfigMapName, metav1.GetOptions{})
	assert.NotNil(t, cm.Data)

	// execute chains upgrade
	err = preUpgradeTektonPruner(ctx, logger, k8sClient, operatorClient, nil)
	assert.NoError(t, err)

	// verify chains with new configMap, map values should be updated
	tc, err = operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, false, *tc.Spec.TektonPruner.Disabled)
	assert.Equal(t, int32(88), *tc.Spec.TektonPruner.GlobalConfig.TTLSecondsAfterFinished)
}

func TestPreUpgradePipelinesAsCodeArtifacts(t *testing.T) {
	t.Setenv("PLATFORM", "openshift")

	tests := []struct {
		name                   string
		tc                     *v1alpha1.TektonConfig
		expectedHubCatalogType string
		expectedHubURL         string
		shouldUpdate           bool
	}{
		{
			name: "PAC enabled with no settings - should update",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.ConfigResourceName,
				},
				Spec: v1alpha1.TektonConfigSpec{
					Platforms: v1alpha1.Platforms{
						OpenShift: v1alpha1.OpenShift{
							PipelinesAsCode: &v1alpha1.PipelinesAsCode{
								Enable: ptr.Bool(true),
							},
						},
					},
				},
			},
			expectedHubCatalogType: "artifacthub",
			expectedHubURL:         "https://artifacthub.io",
			shouldUpdate:           true,
		},
		{
			name: "PAC enabled with tektonhub settings - should update",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.ConfigResourceName,
				},
				Spec: v1alpha1.TektonConfigSpec{
					Platforms: v1alpha1.Platforms{
						OpenShift: v1alpha1.OpenShift{
							PipelinesAsCode: &v1alpha1.PipelinesAsCode{
								Enable: ptr.Bool(true),
								PACSettings: v1alpha1.PACSettings{
									Settings: map[string]string{
										"hub-catalog-type": "tektonhub",
										"hub-url":          "https://api.hub.tekton.dev/v1",
									},
								},
							},
						},
					},
				},
			},
			expectedHubCatalogType: "artifacthub",
			expectedHubURL:         "https://artifacthub.io",
			shouldUpdate:           true,
		},
		{
			name: "PAC enabled with old artifacthub API URL - should update",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.ConfigResourceName,
				},
				Spec: v1alpha1.TektonConfigSpec{
					Platforms: v1alpha1.Platforms{
						OpenShift: v1alpha1.OpenShift{
							PipelinesAsCode: &v1alpha1.PipelinesAsCode{
								Enable: ptr.Bool(true),
								PACSettings: v1alpha1.PACSettings{
									Settings: map[string]string{
										"hub-catalog-type": "artifacthub",
										"hub-url":          "https://artifacthub.io/api/v1",
									},
								},
							},
						},
					},
				},
			},
			expectedHubCatalogType: "artifacthub",
			expectedHubURL:         "https://artifacthub.io",
			shouldUpdate:           true,
		},
		{
			name: "PAC enabled with correct settings - should not update",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.ConfigResourceName,
				},
				Spec: v1alpha1.TektonConfigSpec{
					Platforms: v1alpha1.Platforms{
						OpenShift: v1alpha1.OpenShift{
							PipelinesAsCode: &v1alpha1.PipelinesAsCode{
								Enable: ptr.Bool(true),
								PACSettings: v1alpha1.PACSettings{
									Settings: map[string]string{
										"hub-catalog-type": "artifacthub",
										"hub-url":          "https://artifacthub.io",
									},
								},
							},
						},
					},
				},
			},
			expectedHubCatalogType: "artifacthub",
			expectedHubURL:         "https://artifacthub.io",
			shouldUpdate:           false,
		},
		{
			name: "PAC enabled with hub-catalog-name - should update and remove hub-catalog-name",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.ConfigResourceName,
				},
				Spec: v1alpha1.TektonConfigSpec{
					Platforms: v1alpha1.Platforms{
						OpenShift: v1alpha1.OpenShift{
							PipelinesAsCode: &v1alpha1.PipelinesAsCode{
								Enable: ptr.Bool(true),
								PACSettings: v1alpha1.PACSettings{
									Settings: map[string]string{
										"hub-catalog-type": "artifacthub",
										"hub-url":          "https://artifacthub.io",
										"hub-catalog-name": "tekton",
									},
								},
							},
						},
					},
				},
			},
			expectedHubCatalogType: "artifacthub",
			expectedHubURL:         "https://artifacthub.io",
			shouldUpdate:           true,
		},
		{
			name: "PAC enabled with tektonhub and hub-catalog-name - should update all",
			tc: &v1alpha1.TektonConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.ConfigResourceName,
				},
				Spec: v1alpha1.TektonConfigSpec{
					Platforms: v1alpha1.Platforms{
						OpenShift: v1alpha1.OpenShift{
							PipelinesAsCode: &v1alpha1.PipelinesAsCode{
								Enable: ptr.Bool(true),
								PACSettings: v1alpha1.PACSettings{
									Settings: map[string]string{
										"hub-catalog-type": "tektonhub",
										"hub-url":          "https://api.hub.tekton.dev/v1",
										"hub-catalog-name": "tekton",
									},
								},
							},
						},
					},
				},
			},
			expectedHubCatalogType: "artifacthub",
			expectedHubURL:         "https://artifacthub.io",
			shouldUpdate:           true,
		},
	}

	ctx := context.TODO()
	logger := logging.FromContext(ctx).Named("unit-test")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			operatorClient := operatorFake.NewSimpleClientset()
			k8sClient := k8sFake.NewSimpleClientset()

			err := preUpgradePipelinesAsCodeArtifacts(ctx, logger, k8sClient, operatorClient, nil)
			assert.NoError(t, err)

			// create tektonConfig CR
			if tt.tc != nil {
				_, err = operatorClient.OperatorV1alpha1().TektonConfigs().Create(ctx, tt.tc, metav1.CreateOptions{})
				assert.NoError(t, err)
			}

			// run the upgrade function
			err = preUpgradePipelinesAsCodeArtifacts(ctx, logger, k8sClient, operatorClient, nil)
			assert.NoError(t, err)

			if tt.shouldUpdate {
				tcData, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
				assert.NoError(t, err)
				assert.NotNil(t, tcData.Spec.Platforms.OpenShift.PipelinesAsCode)
				assert.NotNil(t, tcData.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.Settings)

				settings := tcData.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.Settings
				assert.Equal(t, tt.expectedHubCatalogType, settings["hub-catalog-type"])
				assert.Equal(t, tt.expectedHubURL, settings["hub-url"])

				// Verify that hub-catalog-name is removed if it existed in the original settings
				_, hubCatalogNameExists := settings["hub-catalog-name"]
				assert.False(t, hubCatalogNameExists, "hub-catalog-name should be removed from settings")
			}
		})
	}
}

func TestPreUpgradePipelinesAsCodeArtifacts_NonOpenShift(t *testing.T) {
	// Test on non-OpenShift platform
	t.Setenv("PLATFORM", "kubernetes")

	ctx := context.TODO()
	logger := logging.FromContext(ctx).Named("unit-test")
	operatorClient := operatorFake.NewSimpleClientset()
	k8sClient := k8sFake.NewSimpleClientset()

	// Should return early without error on non-OpenShift platform
	err := preUpgradePipelinesAsCodeArtifacts(ctx, logger, k8sClient, operatorClient, nil)
	assert.NoError(t, err)
}

func TestPreUpgradePipelinesAsCodeArtifacts_NoTektonConfig(t *testing.T) {
	t.Setenv("PLATFORM", "openshift")

	ctx := context.TODO()
	logger := logging.FromContext(ctx).Named("unit-test")
	operatorClient := operatorFake.NewSimpleClientset()
	k8sClient := k8sFake.NewSimpleClientset()

	// Should return without error when TektonConfig CR doesn't exist
	err := preUpgradePipelinesAsCodeArtifacts(ctx, logger, k8sClient, operatorClient, nil)
	assert.NoError(t, err)
}

func TestUpdatePipelinesAsCodeConfigMap(t *testing.T) {
	tests := []struct {
		name           string
		configMapData  map[string]string
		expectUpdate   bool
		expectError    bool
		expectedLogMsg string
	}{
		{
			name: "ConfigMap with hub-catalog-name - should remove",
			configMapData: map[string]string{
				"hub-catalog-name": "tekton",
				"other-setting":    "value",
				"hub-url":          "https://artifacthub.io",
			},
			expectUpdate: true,
		},
		{
			name: "ConfigMap with empty hub-catalog-name - should remove",
			configMapData: map[string]string{
				"hub-catalog-name": "",
				"other-setting":    "value",
				"hub-url":          "https://artifacthub.io",
			},
			expectUpdate: true,
		},
		{
			name: "ConfigMap with multiple hub settings but no hub-catalog-name - should not update",
			configMapData: map[string]string{
				"hub-url":          "https://artifacthub.io",
				"hub-catalog-type": "artifacthub",
				"other-setting":    "value",
			},
			expectUpdate: false,
		},
		{
			name: "ConfigMap with only hub-catalog-name - should remove",
			configMapData: map[string]string{
				"hub-catalog-name": "tekton-catalog",
			},
			expectUpdate: true,
		},
		{
			name: "ConfigMap with hub-catalog-name and special characters - should remove",
			configMapData: map[string]string{
				"hub-catalog-name": "tekton-catalog-v1.0.0",
				"secret-name":      "pac-secret",
				"webhook-url":      "https://example.com/webhook",
			},
			expectUpdate: true,
		},
		{
			name: "ConfigMap without hub-catalog-name - should not update",
			configMapData: map[string]string{
				"other-setting": "value",
			},
			expectUpdate: false,
		},
		{
			name:         "ConfigMap with nil data - should not update",
			expectUpdate: false,
		},
		{
			name:          "ConfigMap with empty data - should not update",
			configMapData: map[string]string{},
			expectUpdate:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			logger := logging.FromContext(ctx).Named("unit-test")
			k8sClient := k8sFake.NewSimpleClientset()
			targetNamespace := "test-namespace"

			// Create config map if data is provided
			if tt.configMapData != nil {
				cm := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pipelines-as-code",
						Namespace: targetNamespace,
					},
					Data: tt.configMapData,
				}
				_, err := k8sClient.CoreV1().ConfigMaps(targetNamespace).Create(ctx, cm, metav1.CreateOptions{})
				assert.NoError(t, err)
			} else if tt.name == "ConfigMap with nil data - should not update" {
				// Create config map with nil data
				cm := &v1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "pipelines-as-code",
						Namespace: targetNamespace,
					},
					Data: nil,
				}
				_, err := k8sClient.CoreV1().ConfigMaps(targetNamespace).Create(ctx, cm, metav1.CreateOptions{})
				assert.NoError(t, err)
			}

			// Call the function
			err := updatePipelinesAsCodeConfigMap(ctx, logger, k8sClient, targetNamespace)

			if tt.expectError {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)

			if tt.expectUpdate && tt.configMapData != nil {
				// Verify the config map was updated
				updatedCM, err := k8sClient.CoreV1().ConfigMaps(targetNamespace).Get(ctx, "pipelines-as-code", metav1.GetOptions{})
				assert.NoError(t, err)

				// hub-catalog-name should be removed
				_, exists := updatedCM.Data["hub-catalog-name"]
				assert.False(t, exists, "hub-catalog-name should be removed from config map")

				// Other settings should remain
				if originalValue, hadOtherSetting := tt.configMapData["other-setting"]; hadOtherSetting {
					assert.Equal(t, originalValue, updatedCM.Data["other-setting"])
				}
			}
		})
	}
}

func TestUpdatePipelinesAsCodeConfigMap_NotFound(t *testing.T) {
	ctx := context.TODO()
	logger := logging.FromContext(ctx).Named("unit-test")
	k8sClient := k8sFake.NewSimpleClientset()
	targetNamespace := "test-namespace"

	// Should return without error when config map doesn't exist
	err := updatePipelinesAsCodeConfigMap(ctx, logger, k8sClient, targetNamespace)
	assert.NoError(t, err)
}

func TestUpdateOpenShiftPipelinesAsCodeCR(t *testing.T) {
	tests := []struct {
		name         string
		pacCR        *v1alpha1.OpenShiftPipelinesAsCode
		expectUpdate bool
	}{
		{
			name: "PAC CR with hub-catalog-name - should remove",
			pacCR: &v1alpha1.OpenShiftPipelinesAsCode{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.OpenShiftPipelinesAsCodeName,
				},
				Spec: v1alpha1.OpenShiftPipelinesAsCodeSpec{
					PACSettings: v1alpha1.PACSettings{
						Settings: map[string]string{
							"hub-catalog-name": "tekton",
							"other-setting":    "value",
							"hub-url":          "https://artifacthub.io",
							"hub-catalog-type": "artifacthub",
						},
					},
				},
			},
			expectUpdate: true,
		},
		{
			name: "PAC CR with empty hub-catalog-name - should remove",
			pacCR: &v1alpha1.OpenShiftPipelinesAsCode{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.OpenShiftPipelinesAsCodeName,
				},
				Spec: v1alpha1.OpenShiftPipelinesAsCodeSpec{
					PACSettings: v1alpha1.PACSettings{
						Settings: map[string]string{
							"hub-catalog-name": "",
							"other-setting":    "value",
							"secret-name":      "pac-secret",
						},
					},
				},
			},
			expectUpdate: true,
		},
		{
			name: "PAC CR with hub-catalog-name and complex settings - should remove only hub-catalog-name",
			pacCR: &v1alpha1.OpenShiftPipelinesAsCode{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.OpenShiftPipelinesAsCodeName,
				},
				Spec: v1alpha1.OpenShiftPipelinesAsCodeSpec{
					PACSettings: v1alpha1.PACSettings{
						Settings: map[string]string{
							"hub-catalog-name":    "tekton-catalog-v2",
							"hub-url":             "https://artifacthub.io",
							"hub-catalog-type":    "artifacthub",
							"secret-name":         "pipelines-as-code-secret",
							"webhook-url":         "https://example.com/webhook",
							"application-name":    "Pipelines as Code CI",
							"custom-console-name": "tekton-pipelines",
						},
					},
				},
			},
			expectUpdate: true,
		},
		{
			name: "PAC CR with only hub-catalog-name - should remove",
			pacCR: &v1alpha1.OpenShiftPipelinesAsCode{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.OpenShiftPipelinesAsCodeName,
				},
				Spec: v1alpha1.OpenShiftPipelinesAsCodeSpec{
					PACSettings: v1alpha1.PACSettings{
						Settings: map[string]string{
							"hub-catalog-name": "tekton-only",
						},
					},
				},
			},
			expectUpdate: true,
		},
		{
			name: "PAC CR with multiple hub settings but no hub-catalog-name - should not update",
			pacCR: &v1alpha1.OpenShiftPipelinesAsCode{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.OpenShiftPipelinesAsCodeName,
				},
				Spec: v1alpha1.OpenShiftPipelinesAsCodeSpec{
					PACSettings: v1alpha1.PACSettings{
						Settings: map[string]string{
							"hub-url":          "https://artifacthub.io",
							"hub-catalog-type": "artifacthub",
							"other-setting":    "value",
						},
					},
				},
			},
			expectUpdate: false,
		},
		{
			name: "PAC CR without hub-catalog-name - should not update",
			pacCR: &v1alpha1.OpenShiftPipelinesAsCode{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.OpenShiftPipelinesAsCodeName,
				},
				Spec: v1alpha1.OpenShiftPipelinesAsCodeSpec{
					PACSettings: v1alpha1.PACSettings{
						Settings: map[string]string{
							"other-setting": "value",
						},
					},
				},
			},
			expectUpdate: false,
		},
		{
			name: "PAC CR with nil settings - should not update",
			pacCR: &v1alpha1.OpenShiftPipelinesAsCode{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.OpenShiftPipelinesAsCodeName,
				},
				Spec: v1alpha1.OpenShiftPipelinesAsCodeSpec{
					PACSettings: v1alpha1.PACSettings{
						Settings: nil,
					},
				},
			},
			expectUpdate: false,
		},
		{
			name: "PAC CR with empty settings map - should not update",
			pacCR: &v1alpha1.OpenShiftPipelinesAsCode{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.OpenShiftPipelinesAsCodeName,
				},
				Spec: v1alpha1.OpenShiftPipelinesAsCodeSpec{
					PACSettings: v1alpha1.PACSettings{
						Settings: map[string]string{},
					},
				},
			},
			expectUpdate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.TODO()
			logger := logging.FromContext(ctx).Named("unit-test")
			operatorClient := operatorFake.NewSimpleClientset()

			// Create PAC CR
			_, err := operatorClient.OperatorV1alpha1().OpenShiftPipelinesAsCodes().Create(ctx, tt.pacCR, metav1.CreateOptions{})
			assert.NoError(t, err)

			// Store original settings for comparison
			var originalSettings map[string]string
			if tt.pacCR.Spec.PACSettings.Settings != nil {
				originalSettings = make(map[string]string)
				for k, v := range tt.pacCR.Spec.PACSettings.Settings {
					originalSettings[k] = v
				}
			}

			// Call the function
			err = updateOpenShiftPipelinesAsCodeCR(ctx, logger, operatorClient)
			assert.NoError(t, err)

			if tt.expectUpdate {
				// Verify the PAC CR was updated
				updatedPAC, err := operatorClient.OperatorV1alpha1().OpenShiftPipelinesAsCodes().Get(ctx, v1alpha1.OpenShiftPipelinesAsCodeName, metav1.GetOptions{})
				assert.NoError(t, err)

				// hub-catalog-name should be removed
				_, exists := updatedPAC.Spec.PACSettings.Settings["hub-catalog-name"]
				assert.False(t, exists, "hub-catalog-name should be removed from PAC CR settings")

				// Other settings should remain
				if originalValue, hadOtherSetting := originalSettings["other-setting"]; hadOtherSetting {
					assert.Equal(t, originalValue, updatedPAC.Spec.PACSettings.Settings["other-setting"])
				}
			}
		})
	}
}

func TestUpdateOpenShiftPipelinesAsCodeCR_NotFound(t *testing.T) {
	ctx := context.TODO()
	logger := logging.FromContext(ctx).Named("unit-test")
	operatorClient := operatorFake.NewSimpleClientset()

	// Should return without error when PAC CR doesn't exist
	err := updateOpenShiftPipelinesAsCodeCR(ctx, logger, operatorClient)
	assert.NoError(t, err)
}

func TestRemoveDeprecatedDisableAffinityAssistant(t *testing.T) {
	ctx := context.TODO()
	logger := logging.FromContext(ctx).Named("unit-test")
	operatorClient := operatorFake.NewSimpleClientset()

	// create TektonConfig CR
	tc := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
	}
	_, err := operatorClient.OperatorV1alpha1().TektonConfigs().Create(ctx, tc, metav1.CreateOptions{})
	assert.NoError(t, err)

	// run the upgrade function
	err = removeDeprecatedDisableAffinityAssistant(ctx, logger, nil, operatorClient, nil)
	assert.NoError(t, err)

	// verify the field is removed by checking raw JSON
	tcData, err := operatorClient.OperatorV1alpha1().TektonConfigs().Get(ctx, v1alpha1.ConfigResourceName, metav1.GetOptions{})
	assert.NoError(t, err)

	// convert to unstructured to check raw field
	tcBytes, err := json.Marshal(tcData)
	assert.NoError(t, err)

	var rawTC map[string]interface{}
	err = json.Unmarshal(tcBytes, &rawTC)
	assert.NoError(t, err)

	// check if disable-affinity-assistant field exists in spec.pipeline
	if spec, ok := rawTC["spec"].(map[string]interface{}); ok {
		if pipeline, ok := spec["pipeline"].(map[string]interface{}); ok {
			_, exists := pipeline["disable-affinity-assistant"]
			assert.False(t, exists, "disable-affinity-assistant field should not exist")
		}
	}
}
