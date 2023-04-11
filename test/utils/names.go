/*
Copyright 2020 The Tekton Authors

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

package utils

import "github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"

// ResourceNames holds names of various resources.
type ResourceNames struct {
	TektonPipeline           string
	TektonTrigger            string
	TektonDashboard          string
	TektonAddon              string
	TektonConfig             string
	TektonResult             string
	TektonChain              string
	TektonHub                string
	Namespace                string
	TargetNamespace          string
	OperatorPodSelectorLabel string
}

func GetResourceNames() ResourceNames {
	resourceNames := ResourceNames{
		TektonConfig:             v1alpha1.ConfigResourceName,
		TektonPipeline:           v1alpha1.PipelineResourceName,
		TektonTrigger:            v1alpha1.TriggerResourceName,
		TektonDashboard:          v1alpha1.DashboardResourceName,
		TektonAddon:              v1alpha1.AddonResourceName,
		TektonResult:             v1alpha1.ResultResourceName,
		TektonChain:              v1alpha1.ChainResourceName,
		TektonHub:                v1alpha1.HubResourceName,
		Namespace:                "tekton-operator",
		TargetNamespace:          "tekton-pipelines",
		OperatorPodSelectorLabel: "name=tekton-operator",
	}
	if IsOpenShift() {
		resourceNames.Namespace = "openshift-operators"
		resourceNames.TargetNamespace = "openshift-pipelines"
		resourceNames.OperatorPodSelectorLabel = "name=openshift-pipelines-operator"
	}

	return resourceNames
}
