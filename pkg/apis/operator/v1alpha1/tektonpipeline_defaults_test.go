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
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/test/diff"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func Test_SetDefaults_PipelineProperties(t *testing.T) {
	tp := &TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "namespace",
		},
		Spec: TektonPipelineSpec{
			CommonSpec: CommonSpec{
				TargetNamespace: "namespace",
			},
		},
	}

	properties := PipelineProperties{
		DisableCredsInit:                         ptr.Bool(false),
		AwaitSidecarReadiness:                    ptr.Bool(true),
		RunningInEnvironmentWithInjectedSidecars: ptr.Bool(true),
		RequireGitSshSecretKnownHosts:            ptr.Bool(false),
		EnableCustomTasks:                        ptr.Bool(true),
		EnableApiFields:                          "beta",
		EmbeddedStatus:                           "",
		ScopeWhenExpressionsToTask:               nil,
		SendCloudEventsForRuns:                   ptr.Bool(false),
		VerificationNoMatchPolicy:                config.DefaultNoMatchPolicyConfig,
		EnableProvenanceInStatus:                 ptr.Bool(true),
		EnforceNonfalsifiability:                 config.DefaultEnforceNonfalsifiability,
		EnableKeepPodOnCancel:                    ptr.Bool(config.DefaultEnableKeepPodOnCancel.Enabled),
		ResultExtractionMethod:                   config.DefaultResultExtractionMethod,
		MaxResultSize:                            ptr.Int32(config.DefaultMaxResultSize),
		SetSecurityContext:                       ptr.Bool(config.DefaultSetSecurityContext),
		Coschedule:                               config.DefaultCoschedule,
		DisableInlineSpec:                        config.DefaultDisableInlineSpec,
		EnableCELInWhenExpression:                ptr.Bool(config.DefaultEnableCELInWhenExpression.Enabled),
		EnableStepActions:                        ptr.Bool(config.DefaultEnableStepActions.Enabled),
		EnableParamEnum:                          ptr.Bool(config.DefaultEnableParamEnum.Enabled),
		PipelineMetricsProperties: PipelineMetricsProperties{
			MetricsPipelinerunDurationType: "histogram",
			MetricsPipelinerunLevel:        "pipeline",
			MetricsTaskrunDurationType:     "histogram",
			MetricsTaskrunLevel:            "task",
			CountWithReason:                ptr.Bool(false),
		},
		Resolvers: Resolvers{
			EnableBundlesResolver: ptr.Bool(true),
			EnableHubResolver:     ptr.Bool(true),
			EnableGitResolver:     ptr.Bool(true),
			EnableClusterResolver: ptr.Bool(true),
		},
	}

	tp.SetDefaults(context.TODO())

	if d := cmp.Diff(properties, tp.Spec.PipelineProperties); d != "" {
		t.Errorf("failed to update deployment %s", diff.PrintWantGot(d))
	}
}

// not in use, see: https://github.com/tektoncd/pipeline/pull/7789
// this field is removed from pipeline component
// keeping in types to maintain the API compatibility
// this test verifies that, "EnableTektonOciBundles" always keeps nil on defaults
func TestEnableTektonOciBundlesIgnored(t *testing.T) {
	tp := &TektonPipeline{
		Spec: TektonPipelineSpec{
			Pipeline: Pipeline{
				PipelineProperties: PipelineProperties{
					EnableTektonOciBundles: ptr.Bool(true),
				},
			},
		},
	}
	ctx := context.TODO()

	tests := []struct {
		name                   string
		enableTektonOciBundles *bool
	}{
		{name: "with-true", enableTektonOciBundles: ptr.Bool(true)},
		{name: "with-false", enableTektonOciBundles: ptr.Bool(false)},
		{name: "with-nil", enableTektonOciBundles: nil},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tp.Spec.Pipeline.EnableTektonOciBundles = test.enableTektonOciBundles
			tp.SetDefaults(ctx)
			assert.Nil(t, tp.Spec.Pipeline.EnableTektonOciBundles, "EnableTektonOciBundles removed from pipeline and should be nil on defaulting")
		})
	}
}
