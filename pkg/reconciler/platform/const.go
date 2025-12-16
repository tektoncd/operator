/*
Copyright 2022 The Tekton Authors

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

package platform

// Controllers common to all platforms
const (
	ControllerTektonConfig       ControllerName = "tektonconfig"
	ControllerTektonPipeline     ControllerName = "tektonpipeline"
	ControllerTektonTrigger      ControllerName = "tektontrigger"
	ControllerTektonInstallerSet ControllerName = "tektoninstallerset"
	ControllerTektonHub          ControllerName = "tektonhub"
	ControllerTektonChain        ControllerName = "tektonchain"
	ControllerTektonResult       ControllerName = "tektonresult"
	ControllerManualApprovalGate ControllerName = "manualapprovalgate"
	ControllerTektonPruner       ControllerName = "tektonpruner"
	ControllerTektonScheduler    ControllerName = "tektonscheduler"
	EnvControllerNames           string         = "CONTROLLER_NAMES"
	EnvSharedMainName            string         = "UNIQUE_PROCESS_NAME"
)
