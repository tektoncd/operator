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

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"knative.dev/pkg/ptr"
)

const (
	// openshift specific
	enableMetricsKey          = "enableMetrics"
	enableMetricsDefaultValue = "true"
	ospDefaultSA              = "pipeline"
)

func (tp *TektonPipeline) SetDefaults(ctx context.Context) {
	tp.Spec.setDefaults()
}

func (p *Pipeline) setDefaults() {
	if p.DisableCredsInit == nil {
		p.DisableCredsInit = ptr.Bool(config.DefaultDisableCredsInit)
	}
	if p.AwaitSidecarReadiness == nil {
		p.AwaitSidecarReadiness = ptr.Bool(config.DefaultAwaitSidecarReadiness)
	}
	if p.RunningInEnvironmentWithInjectedSidecars == nil {
		p.RunningInEnvironmentWithInjectedSidecars = ptr.Bool(config.DefaultRunningInEnvWithInjectedSidecars)
	}
	if p.RequireGitSshSecretKnownHosts == nil {
		p.RequireGitSshSecretKnownHosts = ptr.Bool(config.DefaultRequireGitSSHSecretKnownHosts)
	}

	// not in use, see: https://github.com/tektoncd/pipeline/pull/7789
	// this field is removed from pipeline component
	// keeping here to maintain the API compatibility
	p.EnableTektonOciBundles = nil

	if p.EnableCustomTasks == nil {
		// EnableCustomTask is always enable
		p.EnableCustomTasks = ptr.Bool(true)
	}
	if p.SendCloudEventsForRuns == nil {
		p.SendCloudEventsForRuns = ptr.Bool(config.DefaultSendCloudEventsForRuns)
	}
	if p.EnableApiFields == "" {
		p.EnableApiFields = config.DefaultEnableAPIFields
	}

	// "verification-mode" is deprecated and never used.
	// this field will be removed, see https://github.com/tektoncd/operator/issues/1497
	p.VerificationMode = ""

	if p.VerificationNoMatchPolicy == "" {
		p.VerificationNoMatchPolicy = config.DefaultNoMatchPolicyConfig
	}

	if p.EnableProvenanceInStatus == nil {
		p.EnableProvenanceInStatus = ptr.Bool(config.DefaultEnableProvenanceInStatus)
	}

	// Deprecated: set to nil, remove in further release
	p.ScopeWhenExpressionsToTask = nil

	// Deprecated: disable-affinity-assistant is removed from pipeline component
	// set to nil, remove in release-v0.80.x
	p.DisableAffinityAssistant = nil

	if p.EnforceNonfalsifiability == "" {
		p.EnforceNonfalsifiability = config.DefaultEnforceNonfalsifiability
	}

	if p.EnableKeepPodOnCancel == nil {
		p.EnableKeepPodOnCancel = ptr.Bool(config.DefaultEnableKeepPodOnCancel.Enabled)
	}

	if p.ResultExtractionMethod == "" {
		p.ResultExtractionMethod = config.DefaultResultExtractionMethod
	}

	if p.MaxResultSize == nil {
		p.MaxResultSize = ptr.Int32(config.DefaultMaxResultSize)
	}

	if p.SetSecurityContext == nil {
		p.SetSecurityContext = ptr.Bool(config.DefaultSetSecurityContext)
	}

	if p.Coschedule == "" {
		p.Coschedule = config.DefaultCoschedule
	}

	if p.EnableCELInWhenExpression == nil {
		p.EnableCELInWhenExpression = ptr.Bool(config.DefaultEnableCELInWhenExpression.Enabled)
	}

	if p.EnableStepActions == nil {
		p.EnableStepActions = ptr.Bool(config.DefaultFeatureFlags.EnableStepActions)
	}

	if p.EnableParamEnum == nil {
		p.EnableParamEnum = ptr.Bool(config.DefaultEnableParamEnum.Enabled)
	}

	if p.DisableInlineSpec == "" {
		p.DisableInlineSpec = config.DefaultDisableInlineSpec
	}

	if p.MetricsPipelinerunDurationType == "" {
		p.MetricsPipelinerunDurationType = config.DefaultDurationPipelinerunType
	}
	if p.MetricsPipelinerunLevel == "" {
		p.MetricsPipelinerunLevel = config.DefaultPipelinerunLevel
	}
	if p.MetricsTaskrunDurationType == "" {
		p.MetricsTaskrunDurationType = config.DefaultDurationTaskrunType
	}
	if p.MetricsTaskrunLevel == "" {
		p.MetricsTaskrunLevel = config.DefaultTaskrunLevel
	}
	if p.CountWithReason == nil {
		p.CountWithReason = ptr.Bool(false)
	}

	// Resolvers
	if p.EnableBundlesResolver == nil {
		p.EnableBundlesResolver = ptr.Bool(true)
	}
	if p.EnableClusterResolver == nil {
		p.EnableClusterResolver = ptr.Bool(true)
	}
	if p.EnableHubResolver == nil {
		p.EnableHubResolver = ptr.Bool(true)
	}
	if p.EnableGitResolver == nil {
		p.EnableGitResolver = ptr.Bool(true)
	}

	// Statefulset Ordinals
	// if StatefulSet Ordinals mode, buckets should be equal to replicas
	if p.Performance.StatefulsetOrdinals != nil && *p.Performance.StatefulsetOrdinals {
		if p.Performance.Replicas != nil && *p.Performance.Replicas > 1 {
			replicas := uint(*p.Performance.Replicas)
			p.Performance.Buckets = &replicas
		}
	}

	// run platform specific defaulting
	if IsOpenShiftPlatform() {
		p.openshiftDefaulting()
	}
}

func (p *Pipeline) openshiftDefaulting() {
	if p.DefaultServiceAccount == "" {
		p.DefaultServiceAccount = ospDefaultSA
	}

	// Add params with default values if not defined by user
	var found = false
	for i, param := range p.Params {
		if param.Name == enableMetricsKey {
			found = true
			// If the value set is invalid then set key to default value
			if param.Value != "false" && param.Value != "true" {
				p.Params[i].Value = enableMetricsDefaultValue
			}
			break
		}
	}

	if !found {
		p.Params = append(p.Params, Param{
			Name:  enableMetricsKey,
			Value: enableMetricsDefaultValue,
		})
	}
}
