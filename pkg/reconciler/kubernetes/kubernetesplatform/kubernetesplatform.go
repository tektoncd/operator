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

package kubernetesplatform

import (
	"github.com/tektoncd/operator/pkg/reconciler/platform"
)

// KubernetesPlatform defines basic configuration for a Vanila Kubernetes platform
type KubernetesPlatform struct {
	platform.PlatformConfig
	supportedControllers platform.ControllerMap
}

// NewKubernetesPlatform returns an instance of KubernetesPlatform
func NewKubernetesPlatform(pc platform.PlatformConfig) *KubernetesPlatform {
	plt := KubernetesPlatform{
		supportedControllers: kubernetesControllers,
	}
	plt.PlatformConfig = pc
	plt.PlatformConfig.Name = PlatformNameKubernetes
	return &plt
}

// AllSupportedControllers returns a platform.ControllerMap of all controllers (reconcilers) of tektoncd/operator
// supported by Vanila Kubernetes
func (kp *KubernetesPlatform) AllSupportedControllers() platform.ControllerMap {
	return kp.supportedControllers
}

// PlatformParams return platform.PlatformConfig of a KubernetesPlatform
func (kp *KubernetesPlatform) PlatformParams() platform.PlatformConfig {
	return kp.PlatformConfig
}
