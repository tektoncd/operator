/*
Copyright 2026 The Tekton Authors

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

package shipwrightbuild

import (
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common/networkpolicy"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/shipwrightbuild"
)

func TestDefaultPolicies(t *testing.T) {
	policies := defaultPolicies(networkpolicy.OpenShiftPlatformDefaults())

	// One policy per workload: controller and webhook.
	util.AssertEqual(t, len(policies), 2)
	util.AssertEqual(t, policies[0].Name, shipwrightbuild.ControllerDeploymentName)
	util.AssertEqual(t, policies[0].Spec.PodSelector.MatchLabels["name"], shipwrightbuild.ControllerLabels["name"])
	util.AssertEqual(t, policies[1].Name, shipwrightbuild.WebhookDeploymentName)
	util.AssertEqual(t, policies[1].Spec.PodSelector.MatchLabels["name"], shipwrightbuild.WebhookLabels["name"])
}

func TestDefaultDenyPolicy(t *testing.T) {
	deny := defaultDenyPolicy()

	util.AssertEqual(t, deny.Name, "shipwright-build-default-deny")
	util.AssertEqual(t, deny.Spec.PodSelector.MatchLabels[v1alpha1.LabelOperandName], shipwrightbuild.OperandName)
	util.AssertEqual(t, len(deny.Spec.Ingress), 0)
	util.AssertEqual(t, len(deny.Spec.Egress), 0)
}
