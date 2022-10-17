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

	"github.com/tektoncd/pipeline/pkg/apis/config"
)

var (
	// DefaultOpenshiftSA is the default service account for openshift
	DefaultOpenshiftSA = "pipeline"
)

func (tt *TektonTrigger) SetDefaults(ctx context.Context) {
	tt.Spec.Trigger.setDefaults()
}

func (t *Trigger) setDefaults() {
	if t.EnableApiFields == "" {
		t.EnableApiFields = config.DefaultEnableAPIFields
	}

	// run platform specific defaulting
	if IsOpenShiftPlatform() {
		t.openshiftDefaulting()
	}
}

func (t *Trigger) openshiftDefaulting() {
	if t.DefaultServiceAccount == "" {
		t.DefaultServiceAccount = DefaultOpenshiftSA
	}
}
