/*
Copyright 2021 The Tekton Authors

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
	"context"
	"strings"
	"testing"

	"github.com/tektoncd/pruner/pkg/config"
	"gotest.tools/v3/assert"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/ptr"
)

func Test_ValidateTektonConfig_OnDelete(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
		},
	}

	err := tc.Validate(apis.WithinDelete(context.Background()))
	if err != nil {
		t.Errorf("ValidateTektonConfig.Validate() on Delete expected no error, but got one, ValidateTektonConfig: %v", err)
	}
}

func Test_ValidateTektonConfig_MissingTargetNamespace(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			Pruner: Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "missing field(s): spec.targetNamespace", err.Error())
}

func Test_ValidateTektonConfig_InvalidProfile(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "test",
			Pruner:  Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.profile", err.Error())
}

func Test_ValidateTektonConfig_OpenShiftPlatformsOnKubernetes(t *testing.T) {
	t.Setenv("PLATFORM", "")
	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: ConfigResourceName,
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Pruner: Prune{Disabled: true},
			Platforms: Platforms{
				OpenShift: OpenShift{
					PipelinesAsCode: &PipelinesAsCode{Enable: ptr.Bool(true)},
				},
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Assert(t, err != nil)
}

func Test_ValidateTektonConfig_KubernetesPlatformsOnOpenShift(t *testing.T) {
	t.Setenv("PLATFORM", "openshift")
	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: ConfigResourceName,
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Pruner: Prune{Disabled: true},
			Platforms: Platforms{
				Kubernetes: Kubernetes{
					PipelinesAsCode: &PipelinesAsCode{Enable: ptr.Bool(true)},
				},
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Assert(t, err != nil)
}

func Test_ValidateTektonConfig_InvalidPruningResource(t *testing.T) {
	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Pruner: Prune{
				Resources: []string{"task"},
				Schedule:  "",
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "expected exactly one, got neither: spec.pruner.keep, spec.pruner.keep-since\ninvalid value: task: spec.pruner.resources[0]", err.Error())
}

func Test_ValidateTektonConfig_MissingKeepKeepsinceSchedule(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Pruner: Prune{
				Resources: []string{"taskrun"},
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "expected exactly one, got neither: spec.pruner.keep, spec.pruner.keep-since", err.Error())
}

func Test_ValidateTektonConfig_InvalidAddonParam(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Addon: Addon{
				Params: []Param{
					{
						Name:  "invalid-param",
						Value: "val",
					},
				},
			},
			Pruner: Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid key name \"invalid-param\": spec.addon.params", err.Error())
}

func Test_ValidateTektonConfig_InvalidAddonParamValue(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Addon: Addon{
				Params: []Param{
					{
						Name:  "resolverTasks",
						Value: "test",
					},
				},
			},
			Pruner: Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.addon.params.resolverTasks[0]", err.Error())
}

func Test_ValidateTektonConfig_InvalidPipelineProperties(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Pipeline: Pipeline{
				PipelineProperties: PipelineProperties{
					EnableApiFields: "test",
				},
			},
			Pruner: Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.pipeline.enable-api-fields", err.Error())
}

func Test_ValidateTektonConfig_InvalidPipelineOptions(t *testing.T) {
	invalidPolicy := admissionregistrationv1.FailurePolicyType("InvalidPolicy")
	sideEffectUnknown := admissionregistrationv1.SideEffectClassUnknown
	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Pipeline: Pipeline{
				Options: AdditionalOptions{
					WebhookConfigurationOptions: map[string]WebhookConfigurationOptions{
						"validation.webhook.tekton.dev": {
							FailurePolicy: &invalidPolicy,
							SideEffects:   &sideEffectUnknown,
						},
					},
				},
			},
			Pruner: Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: InvalidPolicy: spec.pipeline.options.webhookconfigurationoptions.failurePolicy", err.Error())
}

func Test_ValidateTektonConfig_InvalidManualApprovalOptions(t *testing.T) {
	invalidPolicy := admissionregistrationv1.FailurePolicyType("InvalidPolicy")
	sideEffectUnknown := admissionregistrationv1.SideEffectClassUnknown
	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			ManualApproval: ManualApproval{
				Options: AdditionalOptions{
					WebhookConfigurationOptions: map[string]WebhookConfigurationOptions{
						"validation.webhook.manualapproval.dev": {
							FailurePolicy: &invalidPolicy,
							SideEffects:   &sideEffectUnknown,
						},
					},
				},
			},
			Pruner: Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: InvalidPolicy: spec.manualApproval.options.webhookconfigurationoptions.failurePolicy", err.Error())
}

func Test_ValidateTektonConfig_InvalidTriggerProperties(t *testing.T) {

	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Trigger: Trigger{
				TriggersProperties: TriggersProperties{
					EnableApiFields: "test",
				},
			},
			Pruner: Prune{Disabled: true},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Equal(t, "invalid value: test: spec.trigger.enable-api-fields", err.Error())
}

func Test_ValidateTektonConfig_UpdateTargetNamespace(t *testing.T) {
	ctx := context.Background()
	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Pruner:  Prune{Disabled: true},
		},
	}
	updatedTC := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "test",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "test",
			},
			Profile: "all",
			Pruner:  Prune{Disabled: true},
		},
	}
	ctx = apis.WithinUpdate(ctx, tc)
	err := updatedTC.Validate(ctx)
	assert.Equal(t, `Doesn't allow to update targetNamespace, delete existing TektonConfig and create the updated TektonConfig: spec.targetNamespace`, err.Error())
}

func Test_ValidateTektonConfig_PrunerConfig_Valid(t *testing.T) {
	disabled := false
	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
			Profile: "all",
			// Disable legacy job-based pruner to avoid validation conflicts
			Pruner: Prune{
				Disabled: true,
			},
			// Configure event-based pruner (TektonPruner)
			TektonPruner: Pruner{
				Disabled: &disabled,
				TektonPrunerConfig: TektonPrunerConfig{
					GlobalConfig: &config.GlobalConfig{
						PrunerConfig: config.PrunerConfig{
							SuccessfulHistoryLimit: ptr.Int32(5),
							HistoryLimit:           ptr.Int32(10),
						},
					},
				},
			},
		},
	}

	err := tc.Validate(context.TODO())
	if err != nil {
		t.Errorf("Expected no error for valid pruner config, got: %v", err)
	}
}

func Test_ValidateTektonConfig_PrunerConfig_Invalid(t *testing.T) {
	disabled := false
	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: "config",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
			Profile: "all",
			// Disable legacy job-based pruner to avoid validation conflicts
			Pruner: Prune{
				Disabled: true,
			},
			// Configure event-based pruner (TektonPruner) with invalid config
			TektonPruner: Pruner{
				Disabled: &disabled,
				TektonPrunerConfig: TektonPrunerConfig{
					GlobalConfig: &config.GlobalConfig{
						PrunerConfig: config.PrunerConfig{
							SuccessfulHistoryLimit: ptr.Int32(-1), // Invalid: negative value
						},
					},
				},
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.ErrorContains(t, err, "pruner config validation failed")
}

func Test_ValidateTektonConfig_ResultWatcher(t *testing.T) {
	tc := &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "config",
			Namespace: "namespace",
		},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
			Profile: "all",
			Pruner:  Prune{Disabled: true},
			Result: Result{
				Watcher: ResultsWatcherProperties{
					LabelSelector: ptr.String("not a valid selector=="),
				},
			},
		},
	}

	err := tc.Validate(context.TODO())
	assert.Assert(t, err != nil)
	assert.ErrorContains(t, err, "spec.result.watcher.label_selector")
}

// ---------------------------------------------------------------------------
// NamespaceSyncConfig.validate — namespaceSelector and secretBindings
// ---------------------------------------------------------------------------

func makeSyncTC(ns *NamespaceSyncConfig) *TektonConfig {
	return &TektonConfig{
		ObjectMeta: metav1.ObjectMeta{Name: ConfigResourceName},
		Spec: TektonConfigSpec{
			CommonSpec: CommonSpec{TargetNamespace: "tekton-pipelines"},
			Profile:    "all",
			Pruner:     Prune{Disabled: true},
			Platforms: Platforms{
				OpenShift: OpenShift{NamespaceSync: ns},
			},
		},
	}
}

func Test_ValidateNamespaceSyncConfig_ValidNamespaceSelector(t *testing.T) {
	tc := makeSyncTC(&NamespaceSyncConfig{
		NamespaceSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"pipelines.openshift.io/sync": "true"},
		},
	})
	// Only run this test on OpenShift; on Kubernetes the namespaceSync block is rejected
	// by the platform check, not the selector check.
	t.Setenv("PLATFORM", "openshift")
	err := tc.Validate(context.TODO())
	if err != nil {
		// ignore platform-unrelated errors (SCC cluster calls, etc.)
		assert.Assert(t, !containsFieldError(err, "namespaceSelector"),
			"unexpected namespaceSelector error: %v", err)
	}
}

func Test_ValidateNamespaceSyncConfig_MalformedNamespaceSelector(t *testing.T) {
	tc := makeSyncTC(&NamespaceSyncConfig{
		NamespaceSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      "env",
				Operator: "NotAnOperator", // invalid
			}},
		},
	})
	t.Setenv("PLATFORM", "openshift")
	err := tc.Validate(context.TODO())
	assert.Assert(t, err != nil, "expected validation error for malformed namespaceSelector")
	assert.ErrorContains(t, err, "namespaceSelector")
}

func Test_ValidateNamespaceSyncConfig_MalformedNamespaceSelectorInOperator(t *testing.T) {
	tc := makeSyncTC(&NamespaceSyncConfig{
		NamespaceSelector: &metav1.LabelSelector{
			MatchExpressions: []metav1.LabelSelectorRequirement{{
				Key:      "env",
				Operator: metav1.LabelSelectorOpIn,
				Values:   []string{}, // In with empty values is invalid
			}},
		},
	})
	t.Setenv("PLATFORM", "openshift")
	err := tc.Validate(context.TODO())
	assert.Assert(t, err != nil, "expected validation error for In with empty values")
	assert.ErrorContains(t, err, "namespaceSelector")
}

func Test_ValidateNamespaceSyncConfig_SecretBindingMalformedLabelSelector(t *testing.T) {
	tc := makeSyncTC(&NamespaceSyncConfig{
		SecretBindings: []SecretBinding{{
			LabelSelector: &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      "quay.io/secret",
					Operator: "BadOperator", // invalid
				}},
			},
		}},
	})
	t.Setenv("PLATFORM", "openshift")
	err := tc.Validate(context.TODO())
	assert.Assert(t, err != nil, "expected validation error for malformed secretBindings labelSelector")
	assert.ErrorContains(t, err, "secretBindings[0].labelSelector")
}

func Test_ValidateNamespaceSyncConfig_SecretBindingBothFieldsSet(t *testing.T) {
	tc := makeSyncTC(&NamespaceSyncConfig{
		SecretBindings: []SecretBinding{{
			SecretName:    "my-secret",
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "x"}},
		}},
	})
	t.Setenv("PLATFORM", "openshift")
	err := tc.Validate(context.TODO())
	assert.Assert(t, err != nil, "expected validation error when both secretName and labelSelector are set")
	assert.ErrorContains(t, err, "secretBindings[0]")
}

func Test_ValidateNamespaceSyncConfig_SecretBindingNeitherFieldSet(t *testing.T) {
	tc := makeSyncTC(&NamespaceSyncConfig{
		SecretBindings: []SecretBinding{{}},
	})
	t.Setenv("PLATFORM", "openshift")
	err := tc.Validate(context.TODO())
	assert.Assert(t, err != nil, "expected validation error when neither secretName nor labelSelector is set")
	assert.ErrorContains(t, err, "secretBindings[0]")
}

// containsFieldError reports whether any error in the FieldError tree mentions the given path fragment.
func containsFieldError(err *apis.FieldError, pathFragment string) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), pathFragment)
}
