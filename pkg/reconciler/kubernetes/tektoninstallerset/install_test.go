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

package tektoninstallerset

import (
	"context"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/manifestival/manifestival/fake"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	serviceAccount = namespacedResource("v1", "ServiceAccount", "test", "test-service-account")
	namespace      = namespacedResource("v1", "Namespace", "", "test-service-account")
)

// namespacedResource is an unstructured resource with the given apiVersion, kind, ns and name.
func namespacedResource(apiVersion, kind, ns, name string) unstructured.Unstructured {
	resource := unstructured.Unstructured{}
	resource.SetAPIVersion(apiVersion)
	resource.SetKind(kind)
	resource.SetNamespace(ns)
	resource.SetName(name)
	return resource
}

func TestEnsureResources_CreateResource(t *testing.T) {
	fakeClient := fake.New()
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{serviceAccount}))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	i := NewInstaller(&manifest, fakeClient, logger)

	err = i.EnsureNamespaceScopedResources()
	assert.NilError(t, err)

	res, err := fakeClient.Get(&serviceAccount)
	assert.NilError(t, err)

	assert.Equal(t, res.GetNamespace(), serviceAccount.GetNamespace())
	assert.Equal(t, res.GetName(), serviceAccount.GetName())
}

func TestEnsureResources_UpdateResource(t *testing.T) {
	// service account already exist on cluster
	sa := serviceAccount
	sa.SetAnnotations(map[string]string{
		v1alpha1.LastAppliedHashKey: "abcd",
	})

	fakeClient := fake.New(&sa)
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	// pass an updated sa with some different hash than existing
	newSa := serviceAccount

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{newSa}))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	i := NewInstaller(&manifest, fakeClient, logger)

	err = i.EnsureNamespaceScopedResources()
	assert.NilError(t, err)

	res, err := fakeClient.Get(&serviceAccount)
	assert.NilError(t, err)

	assert.Equal(t, res.GetNamespace(), serviceAccount.GetNamespace())
	assert.Equal(t, res.GetName(), serviceAccount.GetName())
	expectedHash, err := hash.Compute(serviceAccount.Object)
	assert.NilError(t, err)
	assert.Equal(t, res.GetAnnotations()[v1alpha1.LastAppliedHashKey], expectedHash)
}

var (
	readyControllerDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "ready-controller",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentAvailable,
				Status: corev1.ConditionTrue,
			}},
		},
	}
	notReadyControllerDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "not-ready-controller",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentAvailable,
				Status: corev1.ConditionFalse,
			}},
		},
	}
	readyWebhookDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "ready-webhook",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentAvailable,
				Status: corev1.ConditionTrue,
			}},
		},
	}
	notReadyWebhookDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "not-ready-webhook",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentAvailable,
				Status: corev1.ConditionFalse,
			}},
		},
	}
	readyAbcDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "ready-abc",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentAvailable,
				Status: corev1.ConditionTrue,
			}},
		},
	}
	notReadyAbcDeployment = &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "not-ready-abc",
		},
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{{
				Type:   appsv1.DeploymentAvailable,
				Status: corev1.ConditionFalse,
			}},
		},
	}
	completedAbcJob = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "completed-abc",
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobComplete,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}
	failedAbcJob = &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "failed-abc",
		},
		Status: batchv1.JobStatus{
			Conditions: []batchv1.JobCondition{
				{
					Type:   batchv1.JobFailed,
					Status: corev1.ConditionFalse,
				},
			},
		},
	}
)

func TestControllerReady(t *testing.T) {
	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "ready-controller")}

	client := fake.New([]runtime.Object{readyControllerDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, logger)

	err = i.IsControllerReady()
	if err != nil {
		t.Fatal("Unexpected Error: ", err)
	}
}

func TestControllerNotReady(t *testing.T) {
	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "not-ready-controller")}

	client := fake.New([]runtime.Object{notReadyControllerDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, logger)

	err = i.IsControllerReady()
	if err == nil {
		t.Fatal("Expected Error but got nil ")
	}
}

func TestWebhookReady(t *testing.T) {
	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "ready-webhook")}

	client := fake.New([]runtime.Object{readyControllerDeployment, readyWebhookDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, logger)

	err = i.IsWebhookReady()
	if err != nil {
		t.Fatal("Unexpected Error: ", err)
	}
}

func TestWebhookNotReady(t *testing.T) {
	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "not-ready-webhook")}

	client := fake.New([]runtime.Object{readyControllerDeployment, notReadyWebhookDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, logger)

	err = i.IsWebhookReady()
	if err == nil {
		t.Fatal("Expected Error but got nil ")
	}
}

func TestAllDeploymentsReady(t *testing.T) {
	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "ready-abc")}

	client := fake.New([]runtime.Object{readyControllerDeployment, readyWebhookDeployment, readyAbcDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, logger)

	err = i.AllDeploymentsReady()
	if err != nil {
		t.Fatal("Unexpected Error: ", err)
	}
}

func TestAllDeploymentsNotReady(t *testing.T) {
	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "not-ready-abc")}

	client := fake.New([]runtime.Object{readyControllerDeployment, readyWebhookDeployment, notReadyAbcDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, logger)

	err = i.AllDeploymentsReady()
	if err == nil {
		t.Fatal("Expected Error but got nil ")
	}
}

func TestJobCompleted(t *testing.T) {
	in := []unstructured.Unstructured{namespacedResource("batch/v1", "Job", "test", "completed-abc")}

	client := fake.New([]runtime.Object{completedAbcJob}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, logger)

	err = i.IsJobCompleted(context.Background(), nil, "")
	if err != nil {
		t.Fatal("Unexpected Error: ", err)
	}
}

func TestJobFailed(t *testing.T) {
	in := []unstructured.Unstructured{namespacedResource("batch/v1", "Job", "test", "failed-abc")}

	client := fake.New([]runtime.Object{failedAbcJob}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, logger)

	err = i.IsJobCompleted(context.Background(), nil, "")
	if err == nil {
		t.Fatal("Expected Error but got nil ")
	}
}

func TestEnsureResources_DeleteResources(t *testing.T) {
	fakeClient := fake.New(&serviceAccount, &namespace)
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{serviceAccount, namespace}))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	i := NewInstaller(&manifest, fakeClient, logger)

	err = i.DeleteResources()
	assert.NilError(t, err)

	_, err = fakeClient.Get(&serviceAccount)
	assert.Error(t, err, "ServiceAccount \"test-service-account\" not found")

	// namespace must be not deleted
	_, err = fakeClient.Get(&namespace)
	assert.NilError(t, err)
}
