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

import "fmt"

const (
	// operatorVersion
	VersionEnvKey = "VERSION"

	// Profiles
	ProfileAll   = "all"
	ProfileBasic = "basic"
	ProfileLite  = "lite"

	// Addon Params
	ClusterTasksParam      = "clusterTasks"
	PipelineTemplatesParam = "pipelineTemplates"

	// Hub Params
	EnableDevconsoleIntegrationParam = "enable-devconsole-integration"

	ApiFieldAlpha  = "alpha"
	ApiFieldStable = "stable"

	LastAppliedHashKey     = "operator.tekton.dev/last-applied-hash"
	CreatedByKey           = "operator.tekton.dev/created-by"
	ReleaseVersionKey      = "operator.tekton.dev/release-version"
	ReleaseMinorVersionKey = "operator.tekton.dev/release-minor-version"
	TargetNamespaceKey     = "operator.tekton.dev/target-namespace"
	InstallerSetType       = "operator.tekton.dev/type"

	UpgradePending = "upgrade pending"
)

var (
	// RECONCILE_AGAIN_ERR
	// When we updates spec or status we reconcile again and then proceed so
	// that we proceed ahead with updated object
	RECONCILE_AGAIN_ERR = fmt.Errorf("reconcile again and proceed")

	// DEPENDENCY_UPGRADE_PENDING_ERR
	// When a reconciler cannot proceed due to an upgrade in progress of a dependency
	DEPENDENCY_UPGRADE_PENDING_ERR = fmt.Errorf("dependency upgrade pending")

	// VERSION_ENV_NOT_SET_ERR Error when VERSION environment variable is not set
	VERSION_ENV_NOT_SET_ERR = fmt.Errorf("version environment variable %s is not set or empty", VersionEnvKey)
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

	HubParams = map[string]ParamValue{
		EnableDevconsoleIntegrationParam: defaultParamValue,
	}
)

var (
	PipelineResourceName  = "pipeline"
	TriggerResourceName   = "trigger"
	DashboardResourceName = "dashboard"
	AddonResourceName     = "addon"
	ConfigResourceName    = "config"
	ResultResourceName    = "result"
)
