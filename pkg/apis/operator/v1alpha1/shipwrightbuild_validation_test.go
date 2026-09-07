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

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func Test_ValidateShipwrightBuild_ValidName(t *testing.T) {
	sb := &ShipwrightBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name: ShipwrightBuildResourceName,
		},
		Spec: ShipwrightBuildSpec{},
	}

	err := sb.Validate(context.TODO())
	if err != nil {
		t.Errorf("ShipwrightBuild.Validate() expected no error for the valid name, but got one: %v", err)
	}
}

func Test_ValidateShipwrightBuild_InvalidName(t *testing.T) {
	sb := &ShipwrightBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-shipwright-build",
		},
		Spec: ShipwrightBuildSpec{},
	}

	err := sb.Validate(context.TODO())
	if err == nil {
		t.Error("ShipwrightBuild.Validate() expected an error for an invalid name, but got nil")
	}
}

func Test_ValidateShipwrightBuild_OnDelete(t *testing.T) {
	// A wrong name must not block deletion.
	sb := &ShipwrightBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-shipwright-build",
		},
		Spec: ShipwrightBuildSpec{},
	}

	err := sb.Validate(apis.WithinDelete(context.Background()))
	if err != nil {
		t.Errorf("ShipwrightBuild.Validate() on delete expected no error, but got one: %v", err)
	}
}
