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

import (
	"fmt"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/openshift"
)

var (
	TektonPipelineDeploymentLabel  = labelString(v1alpha1.OperandTektoncdPipeline)
	TektonTriggerDeploymentLabel   = labelString(v1alpha1.OperandTektoncdTriggers)
	TektonDashboardDeploymentLabel = labelString(v1alpha1.OperandTektoncdDashboard)
	TektonChainDeploymentLabel     = labelString(v1alpha1.OperandTektoncdChains)
	TektonHubDeploymentLabel       = labelString(v1alpha1.OperandTektoncdHub)
	TektonResultsDeploymentLabel   = labelString(v1alpha1.OperandTektoncdResults)
	TektonAddonDeploymentLabel     = labelString(openshift.OperandOpenShiftPipelinesAddons)
)

func labelString(operandName string) string {
	return fmt.Sprintf("%s=%s", v1alpha1.LabelOperandName, operandName)
}
