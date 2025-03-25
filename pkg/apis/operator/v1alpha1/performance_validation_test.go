/*
Copyright 2025 The Tekton Authors

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
	"strings"
	"testing"

	"knative.dev/pkg/ptr"
)

func uintPtr(u uint) *uint {
	return &u
}

func TestPerformancePropertiesValidate(t *testing.T) {
	tests := []struct {
		name           string
		buckets        uint
		statefulset    *bool
		replicas       *int32
		expectedError  bool
		errorSubstring string
	}{
		{
			name:          "valid: Buckets in range, no statefulset ordinals",
			buckets:       5,
			statefulset:   ptr.Bool(false),
			replicas:      ptr.Int32(3),
			expectedError: false,
		},
		{
			name:          "valid: Buckets in range and statefulset enabled with matching replicas",
			buckets:       3,
			statefulset:   ptr.Bool(true),
			replicas:      ptr.Int32(3),
			expectedError: false,
		},
		{
			name:           "invalid: Buckets below minimum",
			buckets:        0,
			statefulset:    ptr.Bool(false),
			replicas:       ptr.Int32(1),
			expectedError:  true,
			errorSubstring: "buckets",
		},
		{
			name:           "invalid: Buckets above maximum",
			buckets:        11,
			statefulset:    ptr.Bool(false),
			replicas:       ptr.Int32(1),
			expectedError:  true,
			errorSubstring: "buckets",
		},
		{
			name:           "invalid: Statefulset enabled but replicas mismatch",
			buckets:        4,
			statefulset:    ptr.Bool(true),
			replicas:       ptr.Int32(3),
			expectedError:  true,
			errorSubstring: "replicas",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := uintPtr(tc.buckets)
			pp := &PerformanceProperties{
				PerformanceLeaderElectionConfig: PerformanceLeaderElectionConfig{
					Buckets: u,
				},
				PerformanceStatefulsetOrdinalsConfig: PerformanceStatefulsetOrdinalsConfig{
					StatefulsetOrdinals: tc.statefulset,
				},
				Replicas: tc.replicas,
			}

			path := "spec.performance"
			err := pp.Validate(path)

			if tc.expectedError {
				if err == nil {
					t.Errorf("expected an error but got nil")
				} else if tc.errorSubstring != "" && !strings.Contains(err.Error(), tc.errorSubstring) {
					t.Errorf("expected error to contain %q, got: %v", tc.errorSubstring, err)
				}
			} else {
				if err != nil {
					t.Errorf("expected no error, but got: %v", err)
				}
			}
		})
	}
}
