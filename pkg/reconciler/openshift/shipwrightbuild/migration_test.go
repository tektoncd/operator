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

package shipwrightbuild

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorfake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

// scheme is shared across tests; includes core + apps + rbac.
var testScheme = runtime.NewScheme()

func init() {
	_ = corev1.AddToScheme(testScheme)
	_ = appsv1.AddToScheme(testScheme)
	_ = rbacv1.AddToScheme(testScheme)
}

// openshiftBuildCR builds an unstructured OpenShiftBuild CR (the legacy
// operator's top-level CR) with the legacy finalizer set.
const legacyFinalizerName = "operator.openshift.io/openshiftbuilds"

func openshiftBuildCR() *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.openshift.io",
		Version: "v1alpha1",
		Kind:    "OpenShiftBuild",
	})
	obj.SetName("cluster")
	obj.SetFinalizers([]string{legacyFinalizerName})
	return obj
}

func newSB(annotations map[string]string) *v1alpha1.ShipwrightBuild {
	return &v1alpha1.ShipwrightBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:        v1alpha1.ShipwrightBuildResourceName,
			Annotations: annotations,
		},
	}
}

func legacyOperatorDeploymentObj() *appsv1.Deployment {
	replicas := int32(1)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      legacyOperatorDeployment,
			Namespace: legacyNamespace,
		},
		Spec: appsv1.DeploymentSpec{Replicas: &replicas, Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": legacyOperatorDeployment}}},
	}
}

func legacyClusterRole() *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "openshift-builds-build-controller",
			Labels: map[string]string{"app.kubernetes.io/part-of": "openshift-builds"},
		},
	}
}

func legacyClusterRoleBinding() *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "openshift-builds-build-controller",
			Labels: map[string]string{"app.kubernetes.io/part-of": "openshift-builds"},
		},
	}
}

func newDynClient(objs ...*unstructured.Unstructured) *dynfake.FakeDynamicClient {
	runtimeObjs := make([]runtime.Object, len(objs))
	for i, o := range objs {
		runtimeObjs[i] = o
	}
	return dynfake.NewSimpleDynamicClient(testScheme, runtimeObjs...)
}

func TestMigration_RechecksShutdownWhenAnnotationSet(t *testing.T) {
	sb := newSB(map[string]string{migrationAnnotation: migrationDone})
	kubeClient := kubefake.NewSimpleClientset(legacyOperatorDeploymentObj())
	dynClient := newDynClient(openshiftBuildCR())
	operatorClient := operatorfake.NewSimpleClientset(sb)

	if err := migrateFromLegacyInstallation(context.Background(), kubeClient, dynClient, operatorClient, sb); !errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
		t.Fatalf("expected shutdown requeue, got: %v", err)
	}

	// A stale completion annotation must not let the legacy manager restart.
	d, err := kubeClient.AppsV1().Deployments(legacyNamespace).Get(context.Background(), legacyOperatorDeployment, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("expected deployment to still exist: %v", err)
	}
	if *d.Spec.Replicas != 0 {
		t.Errorf("expected replicas=0, got %d", *d.Spec.Replicas)
	}
}

func TestMigration_PreservesFinalizerAndRBACAndStopsOldOperator(t *testing.T) {
	sb := newSB(nil)
	kubeClient := kubefake.NewSimpleClientset(
		legacyOperatorDeploymentObj(),
		legacyClusterRole(),
		legacyClusterRoleBinding(),
	)
	dynClient := newDynClient(openshiftBuildCR())
	operatorClient := operatorfake.NewSimpleClientset(sb)

	if err := migrateFromLegacyInstallation(context.Background(), kubeClient, dynClient, operatorClient, sb); !errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
		t.Fatalf("expected shutdown requeue, got: %v", err)
	}
	if err := migrateFromLegacyInstallation(context.Background(), kubeClient, dynClient, operatorClient, sb); err != nil {
		t.Fatalf("unexpected error after shutdown: %v", err)
	}

	ctx := context.Background()

	// The parent still protects CSI and must not be released for garbage collection.
	cr, err := dynClient.Resource(openshiftBuildGVR).Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("OpenShiftBuild CR should still exist: %v", err)
	}
	if len(cr.GetFinalizers()) != 1 || cr.GetFinalizers()[0] != legacyFinalizerName {
		t.Error("expected legacy finalizer to be preserved")
	}

	// Old operator Deployment scaled to zero.
	d, err := kubeClient.AppsV1().Deployments(legacyNamespace).Get(ctx, legacyOperatorDeployment, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("legacy Deployment should still exist (just scaled down): %v", err)
	}
	if *d.Spec.Replicas != 0 {
		t.Errorf("expected legacy Deployment replicas=0, got %d", *d.Spec.Replicas)
	}

	// Label membership alone never authorizes deleting RBAC.
	if _, err := kubeClient.RbacV1().ClusterRoles().Get(ctx, "openshift-builds-build-controller", metav1.GetOptions{}); err != nil {
		t.Errorf("expected legacy ClusterRole to be preserved: %v", err)
	}
	if _, err := kubeClient.RbacV1().ClusterRoleBindings().Get(ctx, "openshift-builds-build-controller", metav1.GetOptions{}); err != nil {
		t.Errorf("expected legacy ClusterRoleBinding to be preserved: %v", err)
	}

	// Migration annotation stamped on CR.
	updated, err := operatorClient.OperatorV1alpha1().ShipwrightBuilds().Get(ctx, sb.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("failed to get ShipwrightBuild: %v", err)
	}
	if updated.GetAnnotations()[migrationAnnotation] != migrationDone {
		t.Errorf("expected annotation %q=%q, got %v", migrationAnnotation, migrationDone, updated.GetAnnotations())
	}
}

func TestMigration_IdempotentWhenResourcesAlreadyGone(t *testing.T) {
	sb := newSB(nil)
	kubeClient := kubefake.NewSimpleClientset()
	dynClient := newDynClient() // no OpenShiftBuild CR
	operatorClient := operatorfake.NewSimpleClientset(sb)

	if err := migrateFromLegacyInstallation(context.Background(), kubeClient, dynClient, operatorClient, sb); err != nil {
		t.Fatalf("expected no error when legacy resources already absent, got: %v", err)
	}
	for _, action := range operatorClient.Actions() {
		if action.GetVerb() != "get" && action.GetVerb() != "list" {
			t.Fatalf("fresh installation must not stamp migration completion: %v", action)
		}
	}
}

func TestPrepareLegacyInstallation_WaitsForPods(t *testing.T) {
	d := legacyOperatorDeploymentObj()
	zero := int32(0)
	d.Spec.Replicas = &zero
	kubeClient := kubefake.NewSimpleClientset(d, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-manager", Namespace: legacyNamespace, Labels: d.Spec.Selector.MatchLabels},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	})
	if err := prepareLegacyInstallation(context.Background(), kubeClient, newDynClient()); !errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
		t.Fatalf("must wait for manager pods despite zero deployment replicas, got: %v", err)
	}
}

func TestStopLegacyDeployment_WaitsForStatus(t *testing.T) {
	for name, status := range map[string]appsv1.DeploymentStatus{
		"unobserved generation": {ObservedGeneration: 1},
		"replicas":              {ObservedGeneration: 2, Replicas: 1},
		"ready replicas":        {ObservedGeneration: 2, ReadyReplicas: 1},
		"available replicas":    {ObservedGeneration: 2, AvailableReplicas: 1},
		"updated replicas":      {ObservedGeneration: 2, UpdatedReplicas: 1},
	} {
		t.Run(name, func(t *testing.T) {
			d := legacyOperatorDeploymentObj()
			zero := int32(0)
			d.Spec.Replicas = &zero
			d.Generation = 2
			d.Status = status
			client := kubefake.NewSimpleClientset(d)
			if err := stopLegacyOperatorDeployment(context.Background(), client); !errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
				t.Fatalf("must wait for observed shutdown, got: %v", err)
			}
		})
	}
}

func TestStopLegacyDeployment_PropagatesErrors(t *testing.T) {
	for _, stage := range []string{"get", "update", "list pods"} {
		t.Run(stage, func(t *testing.T) {
			d := legacyOperatorDeploymentObj()
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
			if err := stopLegacyOperatorDeployment(context.Background(), client); !errors.Is(err, wantErr) {
				t.Fatalf("expected %v, got: %v", wantErr, err)
			}
		})
	}
}

func legacyCSV() *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "operators.coreos.com/v1alpha1", "kind": "ClusterServiceVersion",
		"metadata": map[string]interface{}{"name": "builds.v1.9.2", "namespace": legacyNamespace},
		"spec": map[string]interface{}{"install": map[string]interface{}{"spec": map[string]interface{}{
			"deployments": []interface{}{
				map[string]interface{}{"name": legacyOperatorDeployment, "spec": map[string]interface{}{"replicas": int64(1)}},
				map[string]interface{}{"name": "unrelated", "spec": map[string]interface{}{"replicas": int64(1)}},
			},
		}}},
	}}
}

func TestPrepareLegacyInstallation_PausesOwningCSV(t *testing.T) {
	for _, ownership := range []string{"reference", "labels"} {
		t.Run(ownership, func(t *testing.T) {
			d := legacyOperatorDeploymentObj()
			if ownership == "reference" {
				d.OwnerReferences = []metav1.OwnerReference{{APIVersion: "operators.coreos.com/v1alpha1", Kind: "ClusterServiceVersion", Name: "builds.v1.9.2"}}
			} else {
				d.Labels = map[string]string{"olm.owner.kind": "ClusterServiceVersion", "olm.owner.namespace": legacyNamespace, "olm.owner": "builds.v1.9.2"}
			}
			kubeClient := kubefake.NewSimpleClientset(d)
			dynClient := newDynClient(legacyCSV())
			ctx := context.Background()
			if err := prepareLegacyInstallation(ctx, kubeClient, dynClient); !errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
				t.Fatalf("expected shutdown requeue, got: %v", err)
			}
			csv, err := dynClient.Resource(legacyCSVGVR).Namespace(legacyNamespace).Get(ctx, "builds.v1.9.2", metav1.GetOptions{})
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
			if err := prepareLegacyInstallation(ctx, kubeClient, dynClient); err != nil {
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

func TestPrepareLegacyInstallation_CSVFailureDoesNotScaleManager(t *testing.T) {
	d := legacyOperatorDeploymentObj()
	d.OwnerReferences = []metav1.OwnerReference{{APIVersion: "operators.coreos.com/v1alpha1", Kind: "ClusterServiceVersion", Name: "builds.v1.9.2"}}
	kubeClient := kubefake.NewSimpleClientset(d)
	dynClient := newDynClient(legacyCSV())
	wantErr := errors.New("CSV update failed")
	dynClient.PrependReactor("update", "clusterserviceversions", func(clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, wantErr
	})
	if err := prepareLegacyInstallation(context.Background(), kubeClient, dynClient); !errors.Is(err, wantErr) {
		t.Fatalf("expected CSV error, got: %v", err)
	}
	for _, action := range kubeClient.Actions() {
		if action.GetVerb() != "get" {
			t.Fatalf("must not mutate manager after CSV failure: %v", action)
		}
	}
}

func TestPrepareLegacyInstallation_RejectsReplacedCSV(t *testing.T) {
	d := legacyOperatorDeploymentObj()
	d.OwnerReferences = []metav1.OwnerReference{{APIVersion: "operators.coreos.com/v1alpha1", Kind: "ClusterServiceVersion", Name: "builds.v1.9.2", UID: "original"}}
	csv := legacyCSV()
	csv.SetUID("replacement")
	kubeClient := kubefake.NewSimpleClientset(d)
	dynClient := newDynClient(csv)
	if err := prepareLegacyInstallation(context.Background(), kubeClient, dynClient); err == nil {
		t.Fatal("must not mutate a CSV that replaced the Deployment's owner")
	}
	for _, action := range append(kubeClient.Actions(), dynClient.Actions()...) {
		if action.GetVerb() != "get" {
			t.Fatalf("owner mismatch must not mutate resources: %v", action)
		}
	}
}

func TestMigration_RechecksUpgradedCSV(t *testing.T) {
	ctx := context.Background()
	sb := newSB(map[string]string{migrationAnnotation: migrationDone})
	d := legacyOperatorDeploymentObj()
	d.OwnerReferences = []metav1.OwnerReference{{APIVersion: "operators.coreos.com/v1alpha1", Kind: "ClusterServiceVersion", Name: "builds.v1.9.3", UID: "upgrade"}}
	csv := legacyCSV()
	csv.SetName("builds.v1.9.3")
	csv.SetUID("upgrade")
	kubeClient := kubefake.NewSimpleClientset(d)
	dynClient := newDynClient(csv, openshiftBuildCR())
	operatorClient := operatorfake.NewSimpleClientset(sb)
	if err := migrateFromLegacyInstallation(ctx, kubeClient, dynClient, operatorClient, sb); !errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
		t.Fatalf("upgrade must trigger shutdown again despite completion annotation, got: %v", err)
	}
	updated, err := dynClient.Resource(legacyCSVGVR).Namespace(legacyNamespace).Get(ctx, csv.GetName(), metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	deployments, _, err := unstructured.NestedSlice(updated.Object, "spec", "install", "spec", "deployments")
	if err != nil {
		t.Fatal(err)
	}
	replicas, _, err := unstructured.NestedInt64(deployments[0].(map[string]interface{}), "spec", "replicas")
	if err != nil || replicas != 0 {
		t.Fatalf("upgraded CSV manager replicas must be zero, got %d; error: %v", replicas, err)
	}
	if len(operatorClient.Actions()) != 0 {
		t.Fatal("must not stamp completion during upgrade shutdown")
	}
}

func TestMigration_RetriesCompletionConflict(t *testing.T) {
	ctx := context.Background()
	sb := newSB(map[string]string{"user-annotation": "preserve"})
	kubeClient := kubefake.NewSimpleClientset()
	dynClient := newDynClient(openshiftBuildCR())
	operatorClient := operatorfake.NewSimpleClientset(sb)
	conflict := true
	operatorClient.PrependReactor("patch", "shipwrightbuilds", func(clienttesting.Action) (bool, runtime.Object, error) {
		if conflict {
			conflict = false
			return true, nil, apierrors.NewConflict(schema.GroupResource{Group: "operator.tekton.dev", Resource: "shipwrightbuilds"}, sb.Name, errors.New("concurrent update"))
		}
		return false, nil, nil
	})
	if err := migrateFromLegacyInstallation(ctx, kubeClient, dynClient, operatorClient, sb); !apierrors.IsConflict(err) {
		t.Fatalf("completion conflict must propagate for retry, got: %v", err)
	}
	if err := migrateFromLegacyInstallation(ctx, kubeClient, dynClient, operatorClient, sb); err != nil {
		t.Fatalf("reconcile after conflict must complete: %v", err)
	}
	updated, err := operatorClient.OperatorV1alpha1().ShipwrightBuilds().Get(ctx, sb.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Annotations[migrationAnnotation] != migrationDone || updated.Annotations["user-annotation"] != "preserve" {
		t.Fatalf("completion patch must preserve unrelated annotations: %v", updated.Annotations)
	}
}

func TestMigration_DetectionFailuresNeverMarkComplete(t *testing.T) {
	for _, wantErr := range []error{
		errors.New("connection refused"),
		apierrors.NewForbidden(schema.GroupResource{Group: "operator.openshift.io", Resource: "openshiftbuilds"}, "cluster", errors.New("denied")),
		apierrors.NewTimeoutError("API timed out", 1),
	} {
		t.Run(wantErr.Error(), func(t *testing.T) {
			sb := newSB(nil)
			dynClient := newDynClient()
			dynClient.PrependReactor("get", "openshiftbuilds", func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, wantErr
			})
			operatorClient := operatorfake.NewSimpleClientset(sb)
			if err := migrateFromLegacyInstallation(context.Background(), kubefake.NewSimpleClientset(), dynClient, operatorClient, sb); !errors.Is(err, wantErr) {
				t.Fatalf("expected detection error, got: %v", err)
			}
			if len(operatorClient.Actions()) != 0 {
				t.Fatal("failed detection must not stamp migration completion")
			}
		})
	}
}

func TestMigration_WaitsForOldShipwrightController(t *testing.T) {
	ctx := context.Background()
	sb := newSB(nil)
	sb.Spec.TargetNamespace = "openshift-pipelines"
	d := legacyOperatorDeploymentObj()
	d.Name = "shipwright-build-controller"
	d.Spec.Selector.MatchLabels["app"] = d.Name
	kubeClient := kubefake.NewSimpleClientset(d)
	dynClient := newDynClient(openshiftBuildCR())
	operatorClient := operatorfake.NewSimpleClientset(sb)
	if err := migrateFromLegacyInstallation(ctx, kubeClient, dynClient, operatorClient, sb); !errors.Is(err, v1alpha1.REQUEUE_EVENT_AFTER) {
		t.Fatalf("expected controller shutdown requeue, got: %v", err)
	}
	if len(operatorClient.Actions()) != 0 {
		t.Fatal("migration must not be stamped complete before controller shutdown")
	}
	if err := migrateFromLegacyInstallation(ctx, kubeClient, dynClient, operatorClient, sb); err != nil {
		t.Fatalf("expected completion after controller shutdown, got: %v", err)
	}
}

func TestMigration_RetiresWebhookOnlyAfterHealthyConversionHandoff(t *testing.T) {
	ctx := context.Background()
	sb := newSB(nil)
	sb.Spec.TargetNamespace = "openshift-pipelines"
	old := legacyOperatorDeploymentObj()
	old.Name = "shipwright-build-webhook"
	replacement := old.DeepCopy()
	replacement.Namespace = sb.Spec.TargetNamespace
	replacement.Status = appsv1.DeploymentStatus{UpdatedReplicas: 1, ReadyReplicas: 1, AvailableReplicas: 1}
	kubeClient := kubefake.NewSimpleClientset(old, replacement)
	objects := []*unstructured.Unstructured{openshiftBuildCR()}
	for _, name := range []string{"builds.shipwright.io", "buildruns.shipwright.io", "buildstrategies.shipwright.io", "clusterbuildstrategies.shipwright.io"} {
		crd := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": "apiextensions.k8s.io/v1", "kind": "CustomResourceDefinition", "metadata": map[string]interface{}{"name": name},
		}}
		assert.NilError(t, unstructured.SetNestedMap(crd.Object, map[string]interface{}{
			"service": map[string]interface{}{"namespace": sb.Spec.TargetNamespace, "name": "shp-build-webhook"}, "caBundle": "injected",
		}, "spec", "conversion", "webhook", "clientConfig"))
		objects = append(objects, crd)
	}
	dynClient := newDynClient(objects...)
	operatorClient := operatorfake.NewSimpleClientset(sb)
	// A single stale CRD route must keep the old server alive.
	crds := schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"}
	stale := objects[1].DeepCopy()
	assert.NilError(t, unstructured.SetNestedField(stale.Object, legacyNamespace, "spec", "conversion", "webhook", "clientConfig", "service", "namespace"))
	_, err := dynClient.Resource(crds).Update(ctx, stale, metav1.UpdateOptions{})
	assert.NilError(t, err)
	assert.Assert(t, errors.Is(migrateFromLegacyInstallation(ctx, kubeClient, dynClient, operatorClient, sb), v1alpha1.REQUEUE_EVENT_AFTER))
	current, err := kubeClient.AppsV1().Deployments(legacyNamespace).Get(ctx, old.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, *current.Spec.Replicas, int32(1))
	_, err = dynClient.Resource(crds).Update(ctx, objects[1], metav1.UpdateOptions{})
	assert.NilError(t, err)
	assert.Assert(t, errors.Is(migrateFromLegacyInstallation(ctx, kubeClient, dynClient, operatorClient, sb), v1alpha1.REQUEUE_EVENT_AFTER))
	assert.NilError(t, migrateFromLegacyInstallation(ctx, kubeClient, dynClient, operatorClient, sb))
	current, err = kubeClient.AppsV1().Deployments(legacyNamespace).Get(ctx, old.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, *current.Spec.Replicas, int32(0))
	completed, err := operatorClient.OperatorV1alpha1().ShipwrightBuilds().Get(ctx, sb.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, completed.Annotations[migrationAnnotation], migrationDone)
}

// TestMigration_NamespacePreserved confirms the openshift-builds namespace is
// NOT deleted, protecting user workloads that may reside there.
func TestMigration_NamespacePreserved(t *testing.T) {
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: legacyNamespace}}
	sb := newSB(nil)
	kubeClient := kubefake.NewSimpleClientset(ns)
	dynClient := newDynClient()
	operatorClient := operatorfake.NewSimpleClientset(sb)

	if err := migrateFromLegacyInstallation(context.Background(), kubeClient, dynClient, operatorClient, sb); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), legacyNamespace, metav1.GetOptions{}); err != nil {
		t.Errorf("expected %s namespace to be preserved, got: %v", legacyNamespace, err)
	}
}

// TestMigration_ShipwrightClusterRolesPreserved confirms that Shipwright
// upstream ClusterRoles (no openshift-builds label) survive; our new install
// will update them via InstallerSet apply semantics.
func TestMigration_ShipwrightClusterRolesPreserved(t *testing.T) {
	shipwrightCR := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "shipwright-build-controller"},
	}
	sb := newSB(nil)
	kubeClient := kubefake.NewSimpleClientset(shipwrightCR)
	dynClient := newDynClient()
	operatorClient := operatorfake.NewSimpleClientset(sb)

	if err := migrateFromLegacyInstallation(context.Background(), kubeClient, dynClient, operatorClient, sb); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := kubeClient.RbacV1().ClusterRoles().Get(context.Background(), "shipwright-build-controller", metav1.GetOptions{}); err != nil {
		t.Errorf("expected shipwright-build-controller ClusterRole to survive migration: %v", err)
	}
}

// TestMigration_CSISCCBindingPreserved verifies that the ClusterRoleBinding and
// ClusterRole granting the legacy CSI driver access to the privileged SCC are
// NOT deleted during migration, even though they carry the legacy label.
// The CSI DaemonSet continues running in openshift-builds until the new
// SharedResource component installs its replacement permissions.
func TestMigration_CSISCCBindingPreserved(t *testing.T) {
	csiBinding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "openshift-builds-shared-resource-node-privileged-binding",
			Labels: map[string]string{"app.kubernetes.io/part-of": "openshift-builds"},
		},
	}
	csiRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:   "openshift-builds-shared-resource-privileged-role",
			Labels: map[string]string{"app.kubernetes.io/part-of": "openshift-builds"},
		},
	}
	sb := newSB(nil)
	kubeClient := kubefake.NewSimpleClientset(csiBinding, csiRole)
	dynClient := newDynClient()
	operatorClient := operatorfake.NewSimpleClientset(sb)

	if err := migrateFromLegacyInstallation(context.Background(), kubeClient, dynClient, operatorClient, sb); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()
	if _, err := kubeClient.RbacV1().ClusterRoleBindings().Get(ctx, csiBinding.Name, metav1.GetOptions{}); err != nil {
		t.Errorf("expected CSI SCC ClusterRoleBinding to be preserved, got: %v", err)
	}
	if _, err := kubeClient.RbacV1().ClusterRoles().Get(ctx, csiRole.Name, metav1.GetOptions{}); err != nil {
		t.Errorf("expected CSI SCC ClusterRole to be preserved, got: %v", err)
	}
}

// TestIsNoCRDError_DoesNotSwallowTransportErrors verifies that network errors
// and other transient failures are NOT treated as "CRD absent".
func TestIsNoCRDError_DoesNotSwallowTransportErrors(t *testing.T) {
	transientErrors := []error{
		fmt.Errorf("connection refused"),
		fmt.Errorf("context deadline exceeded"),
		errors.New("unexpected EOF"),
	}
	for _, err := range transientErrors {
		if isNoCRDError(err) {
			t.Errorf("isNoCRDError(%v) = true; transient errors must not be treated as absent CRD", err)
		}
	}
}

// TestIsNoCRDError_RecognizesMissingCRD verifies that the discovery errors
// returned when the CRD is not registered are correctly identified.
func TestIsNoCRDError_RecognizesMissingCRD(t *testing.T) {
	crdAbsentErrors := []error{
		&apimeta.NoResourceMatchError{PartialResource: schema.GroupVersionResource{Group: "operator.openshift.io", Version: "v1alpha1", Resource: "openshiftbuilds"}},
		&apimeta.NoKindMatchError{GroupKind: schema.GroupKind{Group: "operator.openshift.io", Kind: "OpenShiftBuild"}},
	}
	for _, err := range crdAbsentErrors {
		if !isNoCRDError(err) {
			t.Errorf("isNoCRDError(%T) = false; should be true for missing CRD", err)
		}
	}
}
