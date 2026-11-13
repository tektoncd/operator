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

package legacybuilds

import (
	"context"
	"errors"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func csvClient(objects ...*unstructured.Unstructured) *dynfake.FakeDynamicClient {
	runtimeObjects := make([]runtime.Object, len(objects))
	for i, object := range objects {
		runtimeObjects[i] = object
	}
	return dynfake.NewSimpleDynamicClient(runtime.NewScheme(), runtimeObjects...)
}

func operatorDeployment() *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OperatorDeployment,
			Namespace: Namespace,
		},
		Spec: appsv1.DeploymentSpec{Replicas: &replicas, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": OperatorDeployment}}},
	}
}

func legacyCSV() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operators.coreos.com/v1alpha1", "kind": "ClusterServiceVersion",
		"metadata": map[string]interface{}{"name": "builds.v1.9.2", "namespace": Namespace},
		"spec": map[string]interface{}{"install": map[string]interface{}{"spec": map[string]interface{}{
			"deployments": []interface{}{
				map[string]interface{}{"name": OperatorDeployment, "spec": map[string]interface{}{"replicas": int64(1)}},
				map[string]interface{}{"name": "unrelated", "spec": map[string]interface{}{"replicas": int64(1)}},
			},
		}}},
	}}
}

func TestPrepare_WaitsForPods(t *testing.T) {
	d := operatorDeployment()
	zero := int32(0)
	d.Spec.Replicas = &zero
	kubeClient := kubefake.NewSimpleClientset(d, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-manager", Namespace: Namespace, Labels: d.Spec.Selector.MatchLabels},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	})
	if err := Prepare(context.Background(), kubeClient, csvClient()); !errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
		t.Fatalf("must wait for manager pods despite zero deployment replicas, got: %v", err)
	}
}

func TestStopDeployment_WaitsForStatus(t *testing.T) {
	for name, status := range map[string]appsv1.DeploymentStatus{
		"unobserved generation": {ObservedGeneration: 1},
		"replicas":              {ObservedGeneration: 2, Replicas: 1},
		"ready replicas":        {ObservedGeneration: 2, ReadyReplicas: 1},
		"available replicas":    {ObservedGeneration: 2, AvailableReplicas: 1},
		"updated replicas":      {ObservedGeneration: 2, UpdatedReplicas: 1},
	} {
		t.Run(name, func(t *testing.T) {
			d := operatorDeployment()
			zero := int32(0)
			d.Spec.Replicas = &zero
			d.Generation = 2
			d.Status = status
			client := kubefake.NewSimpleClientset(d)
			if err := StopDeployment(context.Background(), client, OperatorDeployment); !errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
				t.Fatalf("must wait for observed shutdown, got: %v", err)
			}
		})
	}
}

func TestStopDeployment_PropagatesErrors(t *testing.T) {
	for _, stage := range []string{"get", "update", "list pods"} {
		t.Run(stage, func(t *testing.T) {
			d := operatorDeployment()
			if stage == "list pods" {
				zero := int32(0)
				d.Spec.Replicas = &zero
			}
			client := kubefake.NewSimpleClientset(d)
			verb, resource := stage, "deployments"
			if stage == "list pods" {
				verb, resource = "list", "pods"
			}
			wantErr := errors.New("API unavailable")
			client.PrependReactor(verb, resource, func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, wantErr
			})
			if err := StopDeployment(context.Background(), client, OperatorDeployment); !errors.Is(err, wantErr) {
				t.Fatalf("expected %v, got: %v", wantErr, err)
			}
		})
	}
}

func TestPrepare_PausesOwningCSV(t *testing.T) {
	for _, ownership := range []string{"reference", "labels"} {
		t.Run(ownership, func(t *testing.T) {
			d := operatorDeployment()
			if ownership == "reference" {
				d.OwnerReferences = []metav1.OwnerReference{{APIVersion: "operators.coreos.com/v1alpha1", Kind: "ClusterServiceVersion", Name: "builds.v1.9.2"}}
			} else {
				d.Labels = map[string]string{"olm.owner.kind": "ClusterServiceVersion", "olm.owner.namespace": Namespace, "olm.owner": "builds.v1.9.2"}
			}
			kubeClient := kubefake.NewSimpleClientset(d)
			dynClient := csvClient(legacyCSV())
			ctx := context.Background()
			if err := Prepare(ctx, kubeClient, dynClient); !errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
				t.Fatalf("expected shutdown requeue, got: %v", err)
			}
			csv, err := dynClient.Resource(CSVGVR).Namespace(Namespace).Get(ctx, "builds.v1.9.2", metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}
			deployments, _, err := unstructured.NestedSlice(csv.Object, "spec", "install", "spec", "deployments")
			if err != nil {
				t.Fatal(err)
			}
			for i, want := range []int64{0, 1} {
				got, _, err := unstructured.NestedInt64(deployments[i].(map[string]interface{}), "spec", "replicas")
				if err != nil || got != want {
					t.Fatalf("deployment %d replicas=%d, want %d; error: %v", i, got, want, err)
				}
			}
			if err := Prepare(ctx, kubeClient, dynClient); err != nil {
				t.Fatalf("restart after shutdown must be idempotent: %v", err)
			}
			updates := 0
			for _, action := range dynClient.Actions() {
				if action.Matches("update", "clusterserviceversions") {
					updates++
				}
			}
			if updates != 1 {
				t.Fatalf("expected exactly one CSV update, got %d", updates)
			}
		})
	}
}

func TestPrepare_CSVFailureDoesNotScaleManager(t *testing.T) {
	d := operatorDeployment()
	d.OwnerReferences = []metav1.OwnerReference{{APIVersion: "operators.coreos.com/v1alpha1", Kind: "ClusterServiceVersion", Name: "builds.v1.9.2"}}
	kubeClient := kubefake.NewSimpleClientset(d)
	dynClient := csvClient(legacyCSV())
	wantErr := errors.New("CSV update failed")
	dynClient.PrependReactor("update", "clusterserviceversions", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if err := Prepare(context.Background(), kubeClient, dynClient); !errors.Is(err, wantErr) {
		t.Fatalf("expected CSV error, got: %v", err)
	}
	for _, action := range kubeClient.Actions() {
		if action.GetVerb() != "get" {
			t.Fatalf("must not mutate manager after CSV failure: %v", action)
		}
	}
}

func TestPrepare_RejectsReplacedCSV(t *testing.T) {
	d := operatorDeployment()
	d.OwnerReferences = []metav1.OwnerReference{{APIVersion: "operators.coreos.com/v1alpha1", Kind: "ClusterServiceVersion", Name: "builds.v1.9.2", UID: "original"}}
	csv := legacyCSV()
	csv.SetUID("replacement")
	kubeClient := kubefake.NewSimpleClientset(d)
	dynClient := csvClient(csv)
	if err := Prepare(context.Background(), kubeClient, dynClient); err == nil {
		t.Fatal("must not mutate a CSV that replaced the Deployment's owner")
	}
	for _, action := range append(kubeClient.Actions(), dynClient.Actions()...) {
		if action.GetVerb() != "get" {
			t.Fatalf("owner mismatch must not mutate resources: %v", action)
		}
	}
}
