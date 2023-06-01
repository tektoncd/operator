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
		EnableTektonOciBundles:                   ptr.Bool(false),
		EnableCustomTasks:                        ptr.Bool(true),
		EnableApiFields:                          "stable",
		EmbeddedStatus:                           "",
		ScopeWhenExpressionsToTask:               nil,
		SendCloudEventsForRuns:                   ptr.Bool(false),
		VerificationNoMatchPolicy:                config.DefaultNoMatchPolicyConfig,
		EnableProvenanceInStatus:                 ptr.Bool(true),
		PipelineMetricsProperties: PipelineMetricsProperties{
			MetricsPipelinerunDurationType: "histogram",
			MetricsPipelinerunLevel:        "pipeline",
			MetricsTaskrunDurationType:     "histogram",
			MetricsTaskrunLevel:            "task",
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
