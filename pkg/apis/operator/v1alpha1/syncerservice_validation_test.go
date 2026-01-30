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
	"testing"

	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSyncerService_Validate(t *testing.T) {
	ss := &SyncerService{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wrong-name",
			Namespace: "namespace",
		},
	}
	ss.SetDefaults(t.Context())

	err := ss.Validate(t.Context())
	assert.Equal(t, "invalid value: wrong-name: metadata.name, Only one instance of SyncerService is allowed by name, syncer-service", err.Error())
}

func TestSyncerService_ValidateCorrectName(t *testing.T) {
	ss := &SyncerService{
		ObjectMeta: metav1.ObjectMeta{
			Name: "syncer-service",
		},
	}
	ss.SetDefaults(t.Context())

	err := ss.Validate(t.Context())
	assert.Equal(t, err.Error(), "")
}
