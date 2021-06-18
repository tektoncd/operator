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
)

func (ta *TektonAddon) SetDefaults(ctx context.Context) {
	setAddonDefaults(&ta.Spec.Params)
}

func setAddonDefaults(params *[]Param) {

	paramsMap := ParseParams(*params)
	_, ptOk := paramsMap[PipelineTemplatesParam]
	ct, ctOk := paramsMap[ClusterTasksParam]

	// If clusterTasks is false and pipelineTemplate is not set, then set it as false
	// as pipelines templates are created using clusterTasks
	if ctOk && (ct == "false" && !ptOk) {
		*params = append(*params, Param{
			Name:  PipelineTemplatesParam,
			Value: "false",
		})
		paramsMap = ParseParams(*params)
	}

	// set the params with default values if not set in cr
	for d := range AddonParams {
		_, ok := paramsMap[d]
		if !ok {
			*params = append(*params,
				Param{
					Name:  d,
					Value: AddonParams[d].Default,
				})
		}
	}

}
