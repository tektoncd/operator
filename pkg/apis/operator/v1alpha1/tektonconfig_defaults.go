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

func (tc *TektonConfig) SetDefaults(ctx context.Context) {
	if tc.Spec.Profile == "" {
		tc.Spec.Profile = ProfileBasic
	}

	tc.Spec.Pipeline.PipelineProperties.setDefaults()
	tc.Spec.Trigger.TriggersProperties.setDefaults()

	setAddonDefaults(&tc.Spec.Addon)

	// before adding webhook we had default value for pruner's keep as 1
	// but we expect user to define all values now otherwise webhook reject
	// request so if a user has installed prev version and has not enabled
	// pruner then `keep` will have a value 1 and after upgrading
	// to newer version webhook will fail if keep has a value and
	// other fields are not defined
	// this handles that case by removing the default for keep if
	// other pruner fields are not defined
	if len(tc.Spec.Pruner.Resources) == 0 {
		tc.Spec.Pruner.Keep = nil
		tc.Spec.Pruner.Schedule = ""
	} else if tc.Spec.Pruner.Schedule == "" {
		tc.Spec.Pruner.Keep = nil
		tc.Spec.Pruner.Resources = []string{}
	}
}
