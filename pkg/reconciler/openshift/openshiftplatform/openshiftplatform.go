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

package openshiftplatform

import (
	"github.com/tektoncd/operator/pkg/reconciler/platform"
)

// OpenShiftPlatform defines basic configuration for a OpenShift platform
type OpenShiftPlatform struct {
	platform.PlatformConfig
	supportedControllers platform.ControllerMap
}

// NewOpenShiftPlatform returns an instance of OpenShiftPlatform
func NewOpenShiftPlatform(pc platform.PlatformConfig) *OpenShiftPlatform {
	plt := OpenShiftPlatform{
		supportedControllers: openshiftControllers,
	}
	plt.PlatformConfig = pc
	plt.PlatformConfig.Name = PlatformNameOpenShift
	return &plt
}

// AllSupportedControllers returns a platform.ControllerMap of all controllers (reconcilers) of tektoncd/operator
// supported by OpenShift
func (op *OpenShiftPlatform) AllSupportedControllers() platform.ControllerMap {
	return op.supportedControllers
}

// PlatformParams return platform.PlatformConfig of a OpenShiftPlatform
func (op *OpenShiftPlatform) PlatformParams() platform.PlatformConfig {
	return op.PlatformConfig
}
