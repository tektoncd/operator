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

package v1alpha1

import (
	"context"

	"github.com/openshift-pipelines/pipelines-as-code/pkg/params/settings"
)

func (pac *OpenShiftPipelinesAsCode) SetDefaults(ctx context.Context) {
	if pac.Spec.PACSettings.Settings == nil {
		pac.Spec.PACSettings.Settings = map[string]string{}
	}
	setPACDefaults(pac.Spec.PACSettings)
}

func setPACDefaults(set PACSettings) {
	if set.Settings == nil {
		set.Settings = map[string]string{}
	}
	settings.SetDefaults(set.Settings)
}
