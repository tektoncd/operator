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

package common

import (
	"errors"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// mockLister is a generic mock lister for testing
type mockLister[T any] struct {
	items []T
	err   error
}

func (m *mockLister[T]) List(selector labels.Selector) ([]T, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.items, nil
}

func TestComponentListerAdapter(t *testing.T) {
	tests := []struct {
		name          string
		results       []*v1alpha1.TektonResult
		err           error
		expectedCount int
		expectError   bool
	}{
		{
			name:          "Empty list",
			results:       []*v1alpha1.TektonResult{},
			expectedCount: 0,
			expectError:   false,
		},
		{
			name: "Single result",
			results: []*v1alpha1.TektonResult{
				{ObjectMeta: metav1.ObjectMeta{Name: "result-1"}},
			},
			expectedCount: 1,
			expectError:   false,
		},
		{
			name: "Multiple results",
			results: []*v1alpha1.TektonResult{
				{ObjectMeta: metav1.ObjectMeta{Name: "result-1"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "result-2"}},
				{ObjectMeta: metav1.ObjectMeta{Name: "result-3"}},
			},
			expectedCount: 3,
			expectError:   false,
		},
		{
			name:          "Error from underlying lister",
			results:       nil,
			err:           errors.New("lister error"),
			expectedCount: 0,
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockLister[*v1alpha1.TektonResult]{items: tt.results, err: tt.err}
			adapter := TektonResultListerAdapter{Lister: mock}
			resources, err := adapter.List(labels.Everything())

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(resources) != tt.expectedCount {
				t.Errorf("Expected %d resources, got %d", tt.expectedCount, len(resources))
			}

			for i, resource := range resources {
				expectedName := tt.results[i].GetName()
				if resource.GetName() != expectedName {
					t.Errorf("Resource %d: expected name %s, got %s", i, expectedName, resource.GetName())
				}
			}
		})
	}
}

func TestAllListerAdapters(t *testing.T) {
	t.Run("TektonPipelineListerAdapter", func(t *testing.T) {
		mock := &mockLister[*v1alpha1.TektonPipeline]{
			items: []*v1alpha1.TektonPipeline{
				{ObjectMeta: metav1.ObjectMeta{Name: "pipeline-1"}},
			},
		}
		adapter := TektonPipelineListerAdapter{Lister: mock}
		resources, err := adapter.List(labels.Everything())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(resources) != 1 || resources[0].GetName() != "pipeline-1" {
			t.Errorf("Unexpected result: %v", resources)
		}
	})

	t.Run("TektonTriggerListerAdapter", func(t *testing.T) {
		mock := &mockLister[*v1alpha1.TektonTrigger]{
			items: []*v1alpha1.TektonTrigger{
				{ObjectMeta: metav1.ObjectMeta{Name: "trigger-1"}},
			},
		}
		adapter := TektonTriggerListerAdapter{Lister: mock}
		resources, err := adapter.List(labels.Everything())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(resources) != 1 || resources[0].GetName() != "trigger-1" {
			t.Errorf("Unexpected result: %v", resources)
		}
	})

	t.Run("TektonDashboardListerAdapter", func(t *testing.T) {
		mock := &mockLister[*v1alpha1.TektonDashboard]{
			items: []*v1alpha1.TektonDashboard{
				{ObjectMeta: metav1.ObjectMeta{Name: "dashboard-1"}},
			},
		}
		adapter := TektonDashboardListerAdapter{Lister: mock}
		resources, err := adapter.List(labels.Everything())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(resources) != 1 || resources[0].GetName() != "dashboard-1" {
			t.Errorf("Unexpected result: %v", resources)
		}
	})

	t.Run("TektonHubListerAdapter", func(t *testing.T) {
		mock := &mockLister[*v1alpha1.TektonHub]{
			items: []*v1alpha1.TektonHub{
				{ObjectMeta: metav1.ObjectMeta{Name: "hub-1"}},
			},
		}
		adapter := TektonHubListerAdapter{Lister: mock}
		resources, err := adapter.List(labels.Everything())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(resources) != 1 || resources[0].GetName() != "hub-1" {
			t.Errorf("Unexpected result: %v", resources)
		}
	})

	t.Run("TektonChainListerAdapter", func(t *testing.T) {
		mock := &mockLister[*v1alpha1.TektonChain]{
			items: []*v1alpha1.TektonChain{
				{ObjectMeta: metav1.ObjectMeta{Name: "chain-1"}},
			},
		}
		adapter := TektonChainListerAdapter{Lister: mock}
		resources, err := adapter.List(labels.Everything())
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if len(resources) != 1 || resources[0].GetName() != "chain-1" {
			t.Errorf("Unexpected result: %v", resources)
		}
	})
}
