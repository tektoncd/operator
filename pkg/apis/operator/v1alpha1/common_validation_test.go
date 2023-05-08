/*
Copyright 2023 The Tekton Authors

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
	"testing"

	"gotest.tools/v3/assert"
)

func TestValidateCommonTargetNamespace(t *testing.T) {
	cs := &CommonSpec{TargetNamespace: ""}

	tests := []struct {
		name            string
		targetNamespace string
		err             string
		isOpenshift     bool
	}{
		{name: "empty-value", targetNamespace: "", err: "missing field(s): spec.targetNamespace", isOpenshift: false},
		{name: "ns-tekton-pipelines", targetNamespace: "tekton-pipelines", err: "", isOpenshift: false},
		{name: "ns-hello", targetNamespace: "hello", err: "", isOpenshift: false},
		{name: "ns-default", targetNamespace: "default", err: "", isOpenshift: false},
		{name: "ns-openshift-operators", targetNamespace: "openshift-operators", err: "", isOpenshift: false},
		{name: "openshift-ns-openshift-operators", targetNamespace: "openshift-operators", err: "invalid value: openshift-operators: spec.targetNamespace\n'openshift-operators' namespace is not allowed", isOpenshift: true},
		{name: "openshift-ns-openshift-pipelines", targetNamespace: "openshift-pipelines", err: "", isOpenshift: true},
		{name: "openshift-ns-openshift-xyz", targetNamespace: "openshift-xyz", err: "", isOpenshift: true},
		{name: "openshift-ns-tekton-pipelines", targetNamespace: "tekton-pipelines", err: "", isOpenshift: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if test.isOpenshift {
				t.Setenv("PLATFORM", "openshift")
			}
			cs.TargetNamespace = test.targetNamespace
			errs := cs.validate("spec")
			assert.Equal(t, test.err, errs.Error())
		})
	}
}
