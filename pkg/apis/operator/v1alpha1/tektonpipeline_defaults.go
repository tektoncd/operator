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
	enableMetricsKey                         = "enableMetrics"
	enableMetricsDefaultValue                = "true"
	openshiftDefaultDisableAffinityAssistant = true
	ospDefaultSA                             = "pipeline"
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
	if p.EnableTektonOciBundles == nil {
		p.EnableTektonOciBundles = ptr.Bool(config.DefaultEnableTektonOciBundles)
	}
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

	// run platform specific defaulting
	if IsOpenShiftPlatform() {
		p.openshiftDefaulting()
	}
}

func (p *Pipeline) openshiftDefaulting() {
	if p.DefaultServiceAccount == "" {
		p.DefaultServiceAccount = ospDefaultSA
	}

	if p.DisableAffinityAssistant == nil {
		p.DisableAffinityAssistant = ptr.Bool(openshiftDefaultDisableAffinityAssistant)
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
