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

package v1alpha1

import "context"

func (sb *ShipwrightBuild) SetDefaults(ctx context.Context) {
	// Both components default to Enabled when their block is present but the
	// state is unset, mirroring the upstream openshift-builds operator.
	//if sb.Spec != nil && sb.Spec.Shipwright.Build != nil {
	//	sb.Spec.Shipwright.Build.Enable = true
	//}
	//if sb.Spec.SharedResource != nil {
	//	sb.Spec.SharedResource.Enable = true
	//}
}
