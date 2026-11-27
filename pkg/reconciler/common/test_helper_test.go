/*
Copyright 2024 The Tekton Authors

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

package common

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"
)

// toRawExtension converts any Kubernetes object to runtime.RawExtension for testing.
// This helper makes it easy to construct test cases that use AdditionalOptions with RawExtension.
//
// Example usage:
//
//	configMap := corev1.ConfigMap{
//	    ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"foo": "bar"}},
//	    Data: map[string]string{"key": "value"},
//	}
//	options := v1alpha1.AdditionalOptions{
//	    ConfigMaps: map[string]runtime.RawExtension{
//	        "my-configmap": toRawExtension(t, configMap),
//	    },
//	}
func toRawExtension(t *testing.T, obj interface{}) runtime.RawExtension {
	t.Helper()
	data, err := json.Marshal(obj)
	require.NoError(t, err, "failed to marshal object to RawExtension")
	return runtime.RawExtension{Raw: data}
}
