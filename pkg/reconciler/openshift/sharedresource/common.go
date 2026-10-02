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

package sharedresource

const (
	// ControllerDeploymentName is the name of the Deployment that runs the
	// Shared Resource controller.
	ControllerDeploymentName = "shared-resource-controller"
	// OperandName is the operand name label attached to the installed manifests.
	OperandName = "shared-resource"
	// VersionConfigMap is the ConfigMap that carries the installed component
	// version. It must match v1alpha1.SharedResourceResourceName so that the
	// init controller selects the shared-resource payload.
	VersionConfigMap = "shared-resource"
)
