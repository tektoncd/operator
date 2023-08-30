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
	"knative.dev/pkg/ptr"
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
	notReadyStatefulset = &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "notready-statefulset",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    ptr.Int32(1),
			ServiceName: "test",
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      1,
			ReadyReplicas: 0,
		},
	}
	existStatefulset = &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Name:      "exist-statefulset",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    ptr.Int32(1),
			ServiceName: "test",
		},
		Status: appsv1.StatefulSetStatus{
			Replicas:      1,
			ReadyReplicas: 1,
		},
	}
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

func TestEnsureStatefulSetResources(t *testing.T) {
	tests := []struct {
		name      string
		sfsObject *appsv1.StatefulSet
		wantError bool
	}{
		{
			name:      "valid status check",
			sfsObject: existStatefulset,
			wantError: false,
		},
		{
			name:      "invalid status check",
			sfsObject: notReadyStatefulset,
			wantError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sfsObjectFromInstallerSet unstructured.Unstructured
			data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(tt.sfsObject)
			assert.NilError(t, err)
			sfsObjectFromInstallerSet.Object = data
			inExist := []unstructured.Unstructured{sfsObjectFromInstallerSet}
			client := fake.New([]runtime.Object{tt.sfsObject}...)
			manifest, err := mf.ManifestFrom(mf.Slice(inExist), mf.UseClient(client))
			if err != nil {
				t.Fatalf("Failed to generate manifest: %v", err)
			}

			observer, _ := zapobserver.New(zap.InfoLevel)
			logger := zap.New(observer).Sugar()
			i := NewInstaller(&manifest, client, logger)

			err = i.EnsureStatefulSetResources()
			if err != nil != tt.wantError {
				t.Errorf("EnsureStatefulSetResources() error = %v, wantErr %v", err, tt.wantError)
			}
		})
	}
}

// TestEnsureResource tests below scenarios
// 1. ensureResource function create statefulset (assume the data is coming from installerset for the first time)
// 2. User manually update statefulset directly using oc/kubectl client
// 3. again call ensureResource function to make sure user changes are reverted back except changes to replicas other than that direct changes to statefulset is not allowed
func TestEnsureResource(t *testing.T) {
	var sfsObjectFromInstallerSet unstructured.Unstructured
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(existStatefulset)
	assert.NilError(t, err)
	sfsObjectFromInstallerSet.Object = data
	inExist := []unstructured.Unstructured{sfsObjectFromInstallerSet}

	client := fake.New([]runtime.Object{existStatefulset}...)
	manifest, err := mf.ManifestFrom(mf.Slice(inExist), mf.UseClient(client))
	assert.NilError(t, err)

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, logger)

	copyOfInstallerObject := sfsObjectFromInstallerSet.DeepCopy()

	// ensureResource create/update statefulset
	err = i.ensureResource(&sfsObjectFromInstallerSet)
	assert.NilError(t, err)

	// Ensure passed statefulset created/updated by using Get operation
	createdObj, err := i.mfClient.Get(&sfsObjectFromInstallerSet)
	assert.NilError(t, err)
	assert.Assert(t, createdObj != nil)
	formattedData := &appsv1.StatefulSet{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(createdObj.Object, formattedData)
	assert.NilError(t, err)

	if formattedData.Spec.ServiceName != "test" {
		t.Errorf("expected spec.serviceName as test, got %s", formattedData.Spec.ServiceName)
	}

	copyOfInstallerObject.Object = map[string]interface{}{
		"apiVersion": "apps/v1",     // Specify the API version for StatefulSet
		"kind":       "StatefulSet", // Specify the resource kind

		"metadata": map[string]interface{}{
			"name": "exist-statefulset",
		},
		"spec": map[string]interface{}{
			"serviceName": "test-new", // The headless service to use
			"replicas":    3,
		},
	}
	// User manually edit statefulset object
	err = client.Update(copyOfInstallerObject)
	assert.NilError(t, err)

	// Ensure manually updated statefulset is success or not using Get operation
	manuallyUpdatedData, err := client.Get(copyOfInstallerObject)
	assert.NilError(t, err)
	assert.Assert(t, manuallyUpdatedData != nil)
	formattedData = &appsv1.StatefulSet{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manuallyUpdatedData.Object, formattedData)
	assert.NilError(t, err)

	if formattedData.Spec.ServiceName != "test-new" {
		t.Errorf("expected spec.serviceName as test-new, got %s", formattedData.Spec.ServiceName)
	}
	if *formattedData.Spec.Replicas != 3 {
		t.Errorf("expected spec.replicas as 3, got %d", *formattedData.Spec.Replicas)
	}

	// Now installer set will override the manually updated data of statefulset
	err = i.ensureResource(&sfsObjectFromInstallerSet)
	assert.NilError(t, err)
	createdObj, err = i.mfClient.Get(&sfsObjectFromInstallerSet)
	assert.NilError(t, err)
	assert.Assert(t, createdObj != nil)
	formattedData = &appsv1.StatefulSet{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(createdObj.Object, formattedData)
	assert.NilError(t, err)

	if formattedData.Spec.ServiceName != "test" {
		t.Errorf("expected spec.serviceName as test, got %s", formattedData.Spec.ServiceName)
	}
	if *formattedData.Spec.Replicas != 1 {
		t.Errorf("expected spec.replicas as 1, got %d", *formattedData.Spec.Replicas)
	}
}
