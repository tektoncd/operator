/*
Copyright 2020 The Tekton Authors

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
	"context"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/manifestival/manifestival/fake"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCheckDeployments(t *testing.T) {
	readyDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "ready",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentAvailable,
				Status: corev1.ConditionTrue,
			}},
		},
	}

	notReadyDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "notReady",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentAvailable,
				Status: corev1.ConditionFalse,
			}},
		},
	}

	tests := []struct {
		name       string
		inManifest []unstructured.Unstructured
		inAPI      []runtime.Object
		wantError  bool
		wantStatus corev1.ConditionStatus
	}{{
		name: "ready deployment",
		inManifest: []unstructured.Unstructured{
			namespacedResource("apps/v1", "Deployment", "test", "ready"),
		},
		inAPI:      []runtime.Object{readyDeployment},
		wantError:  false,
		wantStatus: corev1.ConditionTrue,
	}, {
		name: "not ready deployment",
		inManifest: []unstructured.Unstructured{
			namespacedResource("apps/v1", "Deployment", "test", "notReady"),
		},
		inAPI:      []runtime.Object{notReadyDeployment},
		wantError:  true,
		wantStatus: corev1.ConditionFalse,
	}, {
		name: "ready and not ready deployment",
		inManifest: []unstructured.Unstructured{
			namespacedResource("apps/v1", "Deployment", "test", "ready"),
			namespacedResource("apps/v1", "Deployment", "test", "notReady"),
		},
		inAPI:      []runtime.Object{},
		wantError:  true,
		wantStatus: corev1.ConditionFalse,
	}, {
		name: "not found deployment",
		inManifest: []unstructured.Unstructured{
			namespacedResource("apps/v1", "Deployment", "test", "notFound"),
		},
		inAPI:      []runtime.Object{},
		wantError:  true,
		wantStatus: corev1.ConditionFalse,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := fake.New(test.inAPI...)
			manifest, err := mf.ManifestFrom(mf.Slice(test.inManifest), mf.UseClient(client))
			if err != nil {
				t.Fatalf("Failed to generate manifest: %v", err)
			}
			tpln := &v1alpha1.TektonTrigger{}
			tpln.Status.InitializeConditions()

			err = CheckDeployments(context.TODO(), &manifest, tpln)
			if (err != nil) != test.wantError {
				t.Fatalf("CheckDeployments() = %v, wantError: %v", err, test.wantError)
			}

			condition := tpln.Status.GetCondition(v1alpha1.DeploymentsAvailable)
			if condition == nil || condition.Status != test.wantStatus {
				t.Fatalf("DeploymentAvailable = %v, want %v", condition, test.wantStatus)
			}
		})
	}
}
