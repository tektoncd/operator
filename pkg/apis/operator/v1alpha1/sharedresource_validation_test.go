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

func Test_ValidateSharedResource_ValidName(t *testing.T) {
	sr := &SharedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: SharedResourceResourceName,
		},
		Spec: SharedResourceSpec{},
	}

	err := sr.Validate(context.TODO())
	if err != nil {
		t.Errorf("SharedResource.Validate() expected no error for the valid name, but got one: %v", err)
	}
}

func Test_ValidateSharedResource_InvalidName(t *testing.T) {
	sr := &SharedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-shared-resource",
		},
		Spec: SharedResourceSpec{},
	}

	err := sr.Validate(context.TODO())
	if err == nil {
		t.Error("SharedResource.Validate() expected an error for an invalid name, but got nil")
	}
}

func Test_ValidateSharedResource_OnDelete(t *testing.T) {
	// A wrong name must not block deletion.
	sr := &SharedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "not-shared-resource",
		},
		Spec: SharedResourceSpec{},
	}

	err := sr.Validate(apis.WithinDelete(context.Background()))
	if err != nil {
		t.Errorf("SharedResource.Validate() on delete expected no error, but got one: %v", err)
	}
}
