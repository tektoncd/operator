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
	"time"

	"knative.dev/pkg/ptr"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	"github.com/manifestival/manifestival/fake"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	"github.com/tektoncd/pipeline/test/diff"
	"go.uber.org/zap"
	zapobserver "go.uber.org/zap/zaptest/observer"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8sfake "k8s.io/client-go/kubernetes/fake"
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
	k8sClient := k8sfake.NewSimpleClientset()
	fakeClient := fake.New()
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{serviceAccount}))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	i := NewInstaller(&manifest, fakeClient, k8sClient, logger)

	err = i.EnsureNamespaceScopedResources()
	assert.NilError(t, err)

	res, err := fakeClient.Get(&serviceAccount)
	assert.NilError(t, err)

	assert.Equal(t, res.GetNamespace(), serviceAccount.GetNamespace())
	assert.Equal(t, res.GetName(), serviceAccount.GetName())
}

func TestEnsureResources_UpdateResource(t *testing.T) {
	k8sClient := k8sfake.NewSimpleClientset()

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

	i := NewInstaller(&manifest, fakeClient, k8sClient, logger)

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

func TestEnsureResources_WaitingDeletion(t *testing.T) {
	k8sClient := k8sfake.NewSimpleClientset()
	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()

	serviceAccount := serviceAccount.DeepCopy()
	serviceAccount.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})

	var saObjectFromInstallerSet unstructured.Unstructured
	data, err := runtime.DefaultUnstructuredConverter.ToUnstructured(serviceAccount)
	assert.NilError(t, err)
	saObjectFromInstallerSet.Object = data
	inExist := []unstructured.Unstructured{saObjectFromInstallerSet}

	fakeClient := fake.New([]runtime.Object{serviceAccount}...)
	manifest, err := mf.ManifestFrom(mf.Slice(inExist), mf.UseClient(fakeClient))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	i := NewInstaller(&manifest, fakeClient, k8sClient, logger)

	// waiting for old resource to be deleted
	err = i.EnsureNamespaceScopedResources()
	assert.Error(t, err, v1alpha1.RECONCILE_AGAIN_ERR.Error())
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
	k8sClient := k8sfake.NewSimpleClientset()

	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "ready-controller")}

	client := fake.New([]runtime.Object{readyControllerDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, k8sClient, logger)

	err = i.IsControllerReady()
	if err != nil {
		t.Fatal("Unexpected Error: ", err)
	}
}

func TestControllerNotReady(t *testing.T) {
	k8sClient := k8sfake.NewSimpleClientset()

	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "not-ready-controller")}

	client := fake.New([]runtime.Object{notReadyControllerDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, k8sClient, logger)

	err = i.IsControllerReady()
	if err == nil {
		t.Fatal("Expected Error but got nil ")
	}
}

func TestWebhookReady(t *testing.T) {
	k8sClient := k8sfake.NewSimpleClientset()

	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "ready-webhook")}

	client := fake.New([]runtime.Object{readyControllerDeployment, readyWebhookDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, k8sClient, logger)

	err = i.IsWebhookReady()
	if err != nil {
		t.Fatal("Unexpected Error: ", err)
	}
}

func TestWebhookNotReady(t *testing.T) {
	k8sClient := k8sfake.NewSimpleClientset()

	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "not-ready-webhook")}

	client := fake.New([]runtime.Object{readyControllerDeployment, notReadyWebhookDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, k8sClient, logger)

	err = i.IsWebhookReady()
	if err == nil {
		t.Fatal("Expected Error but got nil ")
	}
}

func TestAllDeploymentsReady(t *testing.T) {
	k8sClient := k8sfake.NewSimpleClientset()

	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "ready-abc")}

	client := fake.New([]runtime.Object{readyControllerDeployment, readyWebhookDeployment, readyAbcDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, k8sClient, logger)

	err = i.AllDeploymentsReady()
	if err != nil {
		t.Fatal("Unexpected Error: ", err)
	}
}

func TestAllDeploymentsNotReady(t *testing.T) {
	k8sClient := k8sfake.NewSimpleClientset()

	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "not-ready-abc")}

	client := fake.New([]runtime.Object{readyControllerDeployment, readyWebhookDeployment, notReadyAbcDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, k8sClient, logger)

	err = i.AllDeploymentsReady()
	if err == nil {
		t.Fatal("Expected Error but got nil ")
	}
}

func TestJobCompleted(t *testing.T) {
	k8sClient := k8sfake.NewSimpleClientset()

	in := []unstructured.Unstructured{namespacedResource("batch/v1", "Job", "test", "completed-abc")}

	client := fake.New([]runtime.Object{completedAbcJob}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, k8sClient, logger)

	err = i.IsJobCompleted(context.Background(), nil, "")
	if err != nil {
		t.Fatal("Unexpected Error: ", err)
	}
}

func TestJobFailed(t *testing.T) {
	k8sClient := k8sfake.NewSimpleClientset()

	in := []unstructured.Unstructured{namespacedResource("batch/v1", "Job", "test", "failed-abc")}

	client := fake.New([]runtime.Object{failedAbcJob}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	observer, _ := zapobserver.New(zap.InfoLevel)
	logger := zap.New(observer).Sugar()
	i := NewInstaller(&manifest, client, k8sClient, logger)

	err = i.IsJobCompleted(context.Background(), nil, "")
	if err == nil {
		t.Fatal("Expected Error but got nil ")
	}
}

func TestEnsureStatefulSetResources(t *testing.T) {
	ctx := context.TODO()
	k8sClient := k8sfake.NewSimpleClientset()

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
			i := NewInstaller(&manifest, client, k8sClient, logger)

			err = i.EnsureStatefulSetResources(ctx)
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
	ctx := context.TODO()
	k8sClient := k8sfake.NewSimpleClientset()

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
	i := NewInstaller(&manifest, client, k8sClient, logger)

	copyOfInstallerObject := sfsObjectFromInstallerSet.DeepCopy()

	// ensureResource create/update statefulset
	err = i.ensureResource(ctx, &sfsObjectFromInstallerSet)
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
	err = i.ensureResource(ctx, &sfsObjectFromInstallerSet)
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

func TestEnsureResourceWithHPA(t *testing.T) {
	ctx := context.TODO()
	k8sClient := k8sfake.NewSimpleClientset()
	logger, err := zap.NewDevelopment()
	assert.NilError(t, err)

	// get a deployment
	dep1 := getDeployment("foo-1", "bar", 1)

	mfClient := fake.New(dep1)
	i := installer{
		mfClient:      mfClient,
		kubeClientSet: k8sClient,
		logger:        logger.Sugar(),
	}

	_dep1Object, err := runtime.DefaultUnstructuredConverter.ToUnstructured(dep1)
	assert.NilError(t, err)
	unstructuredDep1 := &unstructured.Unstructured{Object: _dep1Object}

	// ensureResource resource created
	err = i.ensureResource(ctx, unstructuredDep1.DeepCopy())
	assert.NilError(t, err)
	existing, err := i.mfClient.Get(unstructuredDep1)
	assert.NilError(t, err)
	if d := cmp.Diff(unstructuredDep1.Object, existing.Object); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}

	// change the replicas count and verify, with no HPA
	expectedCloned := unstructuredDep1.DeepCopy()
	err = unstructured.SetNestedField(expectedCloned.Object, int64(2), "spec", "replicas")
	assert.NilError(t, err)
	err = i.ensureResource(ctx, expectedCloned.DeepCopy())
	assert.NilError(t, err)
	existing, err = i.mfClient.Get(expectedCloned)
	assert.NilError(t, err)
	if d := cmp.Diff(expectedCloned.Object, existing.Object); d != "" {
		t.Errorf("Diff %s", diff.PrintWantGot(d))
	}

	// add HPA and verify the replicas
	hpa1 := getHPA("foo-hpa", dep1.GetNamespace(), 3, dep1.Kind, dep1.GetName())
	_, err = k8sClient.AutoscalingV2().HorizontalPodAutoscalers(dep1.GetNamespace()).Create(ctx, hpa1, metav1.CreateOptions{})
	assert.NilError(t, err)

	hpaConditionsEnabled := []autoscalingv2.HorizontalPodAutoscalerCondition{
		{Type: autoscalingv2.ScalingActive, Status: corev1.ConditionTrue},
	}

	hpaConditionsDisabled := []autoscalingv2.HorizontalPodAutoscalerCondition{
		{Type: autoscalingv2.ScalingActive, Status: corev1.ConditionFalse},
	}

	hpaConditionsUnknown := []autoscalingv2.HorizontalPodAutoscalerCondition{
		{Type: autoscalingv2.ScalingActive, Status: corev1.ConditionUnknown},
	}

	hpaConditionsEmpty := []autoscalingv2.HorizontalPodAutoscalerCondition{}

	tests := []struct {
		name             string
		hpaConditions    []autoscalingv2.HorizontalPodAutoscalerCondition
		desiredReplicas  int32
		minReplicas      *int32
		maxReplicas      int32
		manifestReplicas *int64 // unstructured.SetNestedField not accepts int32
		expectedReplicas int32
	}{
		{
			name:             "test-hpa_enabled-desired_replicas_1",
			hpaConditions:    hpaConditionsEnabled,
			desiredReplicas:  1,
			minReplicas:      ptr.Int32(1),
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(3),
			expectedReplicas: 1,
		},
		{
			name:             "test-hpa_enabled-desired_replicas_3",
			hpaConditions:    hpaConditionsEnabled,
			desiredReplicas:  3,
			minReplicas:      ptr.Int32(1),
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(6),
			expectedReplicas: 3,
		},
		{
			name:             "test-hpa_enabled-desired_replicas_0-min_replicas_1",
			hpaConditions:    hpaConditionsEnabled,
			desiredReplicas:  0,
			minReplicas:      ptr.Int32(1),
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(3),
			expectedReplicas: 1,
		},
		{
			name:             "test-hpa_enabled-desired_replicas_0-min_replicas_2",
			hpaConditions:    hpaConditionsEnabled,
			desiredReplicas:  0,
			minReplicas:      ptr.Int32(2),
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(3),
			expectedReplicas: 2,
		},
		{
			name:             "test-hpa_enabled-desired_replicas_0-min_replicas_nil-manifest_replicas_3",
			hpaConditions:    hpaConditionsEnabled,
			desiredReplicas:  0,
			minReplicas:      nil,
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(3),
			expectedReplicas: 1,
		},
		{
			name:             "test-hpa_unknown-desired_replicas_1",
			hpaConditions:    hpaConditionsUnknown,
			desiredReplicas:  1,
			minReplicas:      ptr.Int32(1),
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(3),
			expectedReplicas: 1,
		},
		{
			name:             "test-hpa_unknown-desired_replicas_0-min_replicas_2",
			hpaConditions:    hpaConditionsUnknown,
			desiredReplicas:  0,
			minReplicas:      ptr.Int32(2),
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(3),
			expectedReplicas: 2,
		},
		{
			name:             "test-hpa_unknown-desired_replicas_0-min_replicas_2",
			hpaConditions:    hpaConditionsUnknown,
			desiredReplicas:  0,
			minReplicas:      ptr.Int32(2),
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(3),
			expectedReplicas: 2,
		},
		{
			name:             "test-hpa_unknown-desired_replicas_0-min_replicas_nil-manifest_replicas_3",
			hpaConditions:    hpaConditionsUnknown,
			desiredReplicas:  0,
			minReplicas:      nil,
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(3),
			expectedReplicas: 1,
		},
		{
			name:             "test-hpa_disabled-manifest_replicas_1",
			hpaConditions:    hpaConditionsDisabled,
			desiredReplicas:  2,
			minReplicas:      ptr.Int32(2),
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(1),
			expectedReplicas: 2,
		},
		{
			name:             "test_hpa-disabled-manifest_replicas_3",
			hpaConditions:    hpaConditionsDisabled,
			desiredReplicas:  2,
			minReplicas:      ptr.Int32(2),
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(3),
			expectedReplicas: 3,
		},
		{
			name:             "test_hpa-disabled-manifest_replicas_8",
			hpaConditions:    hpaConditionsDisabled,
			desiredReplicas:  2,
			minReplicas:      ptr.Int32(2),
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(8),
			expectedReplicas: 5,
		},
		{
			name:             "test_hpa-disabled-manifest_replicas_5",
			hpaConditions:    hpaConditionsDisabled,
			desiredReplicas:  2,
			minReplicas:      nil,
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(5),
			expectedReplicas: 5,
		},
		{
			name:             "test_hpa-disabled-manifest_replicas_nil",
			hpaConditions:    hpaConditionsDisabled,
			desiredReplicas:  2,
			minReplicas:      nil,
			maxReplicas:      5,
			manifestReplicas: nil,
			expectedReplicas: 1,
		},
		{
			name:             "test_hpa-disabled-manifest_replicas_nil-min_replicas_2",
			hpaConditions:    hpaConditionsDisabled,
			desiredReplicas:  2,
			minReplicas:      ptr.Int32(2),
			maxReplicas:      5,
			manifestReplicas: nil,
			expectedReplicas: 2,
		},
		{
			name:             "test-hpa_empty-manifest_replicas_1",
			hpaConditions:    hpaConditionsEmpty,
			desiredReplicas:  2,
			minReplicas:      ptr.Int32(2),
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(1),
			expectedReplicas: 2,
		},
		{
			name:             "test-hpa_empty-manifest_replicas_3",
			hpaConditions:    hpaConditionsEmpty,
			desiredReplicas:  1,
			minReplicas:      ptr.Int32(1),
			maxReplicas:      5,
			manifestReplicas: ptr.Int64(3),
			expectedReplicas: 3,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// update hpa values
			hpa1.Status.DesiredReplicas = test.desiredReplicas
			hpa1.Spec.MinReplicas = test.minReplicas
			hpa1.Spec.MaxReplicas = test.maxReplicas
			hpa1.Status.Conditions = test.hpaConditions
			_, err = k8sClient.AutoscalingV2().HorizontalPodAutoscalers(dep1.GetNamespace()).Update(ctx, hpa1, metav1.UpdateOptions{})
			assert.NilError(t, err)

			expectedCloned = unstructuredDep1.DeepCopy()
			// update manifest replicas in expected object
			if test.manifestReplicas == nil {
				unstructured.RemoveNestedField(expectedCloned.Object, "spec", "replicas")
			} else {
				err = unstructured.SetNestedField(expectedCloned.Object, *test.manifestReplicas, "spec", "replicas")
				assert.NilError(t, err)
			}

			err = i.ensureResource(ctx, expectedCloned.DeepCopy())
			assert.NilError(t, err)

			// update the count in expected to expectedReplicas count, to verify the updated deployment
			_expected := expectedCloned.DeepCopy()
			err = unstructured.SetNestedField(_expected.Object, int64(test.expectedReplicas), "spec", "replicas")
			assert.NilError(t, err)
			existing, err = i.mfClient.Get(expectedCloned)
			assert.NilError(t, err)
			if d := cmp.Diff(_expected.Object, existing.Object); d != "" {
				t.Errorf("Diff %s", diff.PrintWantGot(d))
			}
		})
	}

	// remove the desired replicas count from HPA

}

func TestEnsureResourceWaitingDeletion(t *testing.T) {
	ctx := context.TODO()
	k8sClient := k8sfake.NewSimpleClientset()

	existStatefulset := existStatefulset.DeepCopy()
	existStatefulset.SetDeletionTimestamp(&metav1.Time{Time: time.Now()})

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
	i := NewInstaller(&manifest, client, k8sClient, logger)

	// waiting for old statefulset to be deleted
	err = i.ensureResource(ctx, &sfsObjectFromInstallerSet)
	assert.Error(t, err, v1alpha1.RECONCILE_AGAIN_ERR.Error())
}

func getDeployment(name, namespace string, replicas int32) *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind: "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   namespace,
			Annotations: map[string]string{"name": name},
			Labels:      map[string]string{"name": name},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.Int32(replicas),
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{},
				},
			},
		},
	}
}

func getHPA(name, namespace string, desiredReplicas int32, targetKind, targetName string) *autoscalingv2.HorizontalPodAutoscaler {
	return &autoscalingv2.HorizontalPodAutoscaler{
		TypeMeta: metav1.TypeMeta{
			Kind: "HorizontalPodAutoscaler",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: autoscalingv2.HorizontalPodAutoscalerSpec{
			ScaleTargetRef: autoscalingv2.CrossVersionObjectReference{
				Kind: targetKind,
				Name: targetName,
			},
			MinReplicas: ptr.Int32(1),
			MaxReplicas: int32(4),
		},
		Status: autoscalingv2.HorizontalPodAutoscalerStatus{
			CurrentReplicas: 3,
			DesiredReplicas: 3,
			Conditions:      []autoscalingv2.HorizontalPodAutoscalerCondition{},
		},
	}
}
