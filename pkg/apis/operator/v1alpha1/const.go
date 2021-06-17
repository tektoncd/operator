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

const (
	// Profiles
	ProfileAll   = "all"
	ProfileBasic = "basic"
	ProfileLite  = "lite"

	// Addon Params
	ClusterTasksParam      = "clusterTasks"
	PipelineTemplatesParam = "pipelineTemplates"
)

var (
	defaultParamValue = ParamValue{
		Default:  "true",
		Possible: []string{"true", "false"},
	}

	// Profiles
	Profiles = []string{
		ProfileLite,
		ProfileBasic,
		ProfileAll,
	}

	PruningResource = []string{
		"taskrun",
		"pipelinerun",
	}

	AddonParams = map[string]ParamValue{
		ClusterTasksParam:      defaultParamValue,
		PipelineTemplatesParam: defaultParamValue,
	}
)
