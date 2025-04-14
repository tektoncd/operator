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

package common

import (
	"path"
	"reflect"
	"testing"

	mf "github.com/manifestival/manifestival"
	"gotest.tools/v3/assert"
	"knative.dev/pkg/ptr"
)

func TestFetchVersionFromConfigMap(t *testing.T) {

	testData := path.Join("testdata", "test-fetch-version-from-configmap.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	version, err := FetchVersionFromConfigMap(manifest, "pipelines-info")
	if err != nil {
		t.Fatal("Unexpected Error: ", err)
	}

	if version != "devel" {
		t.Fatal("invalid label fetched from crd: ", version)
	}
}

func TestFetchVersionFromConfigMap_ConfigMapNotFound(t *testing.T) {

	testData := path.Join("testdata", "test-fetch-version-from-configmap.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	_, err = FetchVersionFromConfigMap(manifest, "triggers-info")
	if err == nil {
		t.Fatal("Expected error found nil")
	}

	assert.Error(t, err, errConfigMap.Error())
}

func TestFetchVersionFromConfigMap_VersionKeyNotFound(t *testing.T) {

	testData := path.Join("testdata", "test-fetch-version-from-configmap-invalid.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assertNoError(t, err)

	_, err = FetchVersionFromConfigMap(manifest, "pipelines-info")
	if err == nil {
		t.Fatal("Expected error found nil")
	}

	assert.Error(t, err, errConfigMap.Error())
}

func TestStructMap(t *testing.T) {
	in := struct {
		StringValue  string  `json:"str"`
		Int32Ptr     *int32  `json:"int32Ptr"`
		IntValue     int     `json:"intValue"`
		Float32Value float32 `json:"float32_value"`
		BoolValue    bool    `json:"bool-value"`
	}{
		StringValue:  "hi",
		Int32Ptr:     ptr.Int32(1),
		IntValue:     2,
		Float32Value: 2.200001,
		BoolValue:    false,
	}

	// json Unmarshal converts all the number types into float64
	expectedOut := map[string]interface{}{
		"str":           "hi",
		"int32Ptr":      float64(1),
		"intValue":      float64(2),
		"float32_value": float64(2.200001),
		"bool-value":    false,
	}

	actualOut := map[string]interface{}{}

	err := StructToMap(&in, &actualOut)
	assert.NilError(t, err)
	assert.Check(t, reflect.DeepEqual(actualOut, expectedOut), actualOut)
}

func TestStructMapError(t *testing.T) {
	in := struct {
		StringValue string `json:"str"`
	}{
		StringValue: "hi",
	}

	actualOut := map[string]interface{}{}

	err := StructToMap(&in, actualOut)
	assert.Error(t, err, "json: Unmarshal(non-pointer map[string]interface {})")
}

func TestSerializeLabelsToJSON(t *testing.T) {
	// Test cases with different inputs
	tests := []struct {
		name           string
		labels         map[string]string
		expectedOutput string
		expectError    bool
	}{
		{
			name: "Valid input with multiple labels",
			labels: map[string]string{
				"app":   "my-app",
				"env":   "production",
				"owner": "dev-team",
			},
			expectedOutput: `{"app":"my-app","env":"production","owner":"dev-team"}`,
			expectError:    false,
		},
		{
			name:           "Empty input",
			labels:         map[string]string{},
			expectedOutput: `{}`,
			expectError:    false,
		},
		{
			name: "Single label",
			labels: map[string]string{
				"key": "value",
			},
			expectedOutput: `{"key":"value"}`,
			expectError:    false,
		},
		{
			name: "Special characters in keys and values",
			labels: map[string]string{
				"foo@bar":   "bazqux",
				"key#1":     "value$%",
				"space key": "with space",
			},
			expectedOutput: `{"foo@bar":"bazqux","key#1":"value$%","space key":"with space"}`,
			expectError:    false,
		},
	}

	// Loop over each test case
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function with the test labels
			actual, err := SerializeLabelsToJSON(tt.labels)

			// Check if we expect an error
			if tt.expectError {
				// Assert that an error was returned
				assert.Error(t, err, "Expected error, but got none")
			} else {
				// Check if the output matches the expected result
				assert.Equal(t, tt.expectedOutput, actual)
			}
		})
	}
}
