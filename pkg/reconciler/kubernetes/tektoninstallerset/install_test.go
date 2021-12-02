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
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	"github.com/manifestival/manifestival/fake"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	namespace          = clusterScopedResource("v1", "Namespace", "test-ns")
	clusterRole        = clusterScopedResource("rbac.authorization.k8s.io/v1", "ClusterRole", "test-cluster-role")
	role               = namespacedResource("rbac.authorization.k8s.io/v1", "Role", "test", "test-role")
	serviceAccount     = namespacedResource("v1", "ServiceAccount", "test", "test-service-account")
	clusterRoleBinding = clusterScopedResource("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "test-cluster-role-binding")
	roleBinding        = namespacedResource("rbac.authorization.k8s.io/v1", "RoleBinding", "test", "test-role-binding")
	crd                = clusterScopedResource("apiextensions.k8s.io/v1", "CustomResourceDefinition", "test-crd")
	secret             = namespacedResource("v1", "Secret", "test", "test-secret")
	validatingWebhook  = clusterScopedResource("admissionregistration.k8s.io/v1", "ValidatingWebhookConfiguration", "test-validating-webhook")
	mutatingWebhook    = clusterScopedResource("admissionregistration.k8s.io/v1", "MutatingWebhookConfiguration", "test-mutating-webhook")
	configMap          = namespacedResource("v1", "ConfigMap", "test", "test-configmap")
	deployment         = namespacedResource("apps/v1", "Deployment", "test", "test-deployment")
	service            = namespacedResource("v1", "Service", "test", "test-service")
	hpa                = namespacedResource("autoscaling/v2beta1", "HorizontalPodAutoscaler", "test", "test-hpa")
)

type fakeClient struct {
	err            error
	getErr         error
	createErr      error
	resourcesExist bool
	gets           []unstructured.Unstructured
	creates        []unstructured.Unstructured
	deletes        []unstructured.Unstructured
}

func (f *fakeClient) Get(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var resource *unstructured.Unstructured
	if f.resourcesExist {
		for _, item := range f.gets {
			if obj.GetKind() == item.GetKind() && obj.GetName() == item.GetName() {
				return &item, nil
			}
		}

	}
	return resource, f.getErr
}

func (f *fakeClient) Delete(obj *unstructured.Unstructured, options ...mf.DeleteOption) error {
	f.deletes = append(f.deletes, *obj)
	return f.err
}

func (f *fakeClient) Create(obj *unstructured.Unstructured, options ...mf.ApplyOption) error {
	obj.SetAnnotations(nil) // Deleting the extra annotation. Irrelevant for the test.
	f.creates = append(f.creates, *obj)
	return f.createErr
}

func (f *fakeClient) Update(obj *unstructured.Unstructured, options ...mf.ApplyOption) error {
	return f.err
}

// namespacedResource is an unstructured resource with the given apiVersion, kind, ns and name.
func namespacedResource(apiVersion, kind, ns, name string) unstructured.Unstructured {
	resource := unstructured.Unstructured{}
	resource.SetAPIVersion(apiVersion)
	resource.SetKind(kind)
	resource.SetNamespace(ns)
	resource.SetName(name)
	return resource
}

// clusterScopedResource is an unstructured resource with the given apiVersion, kind and name.
func clusterScopedResource(apiVersion, kind, name string) unstructured.Unstructured {
	return namespacedResource(apiVersion, kind, "", name)
}

func TestInstaller(t *testing.T) {
	crd.SetDeletionTimestamp(&metav1.Time{})
	in := []unstructured.Unstructured{namespace, deployment, clusterRole, role,
		roleBinding, clusterRoleBinding, serviceAccount, crd, validatingWebhook, mutatingWebhook, configMap, service, hpa, secret}

	client := &fakeClient{}
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	i := installer{
		Manifest: manifest,
	}

	want := []unstructured.Unstructured{crd}

	err = i.EnsureCRDs()
	if err != nil {
		t.Fatal("Unexpected Error while installing resources: ", err)
	}

	if len(want) != len(client.creates) {
		t.Fatalf("Unexpected creates: %s", fmt.Sprintf("(-got, +want): %s", cmp.Diff(client.creates, want)))
	}

	// reset created array
	client.creates = []unstructured.Unstructured{}

	want = []unstructured.Unstructured{namespace, clusterRole, validatingWebhook, mutatingWebhook}

	err = i.EnsureClusterScopedResources()
	if err != nil {
		t.Fatal("Unexpected Error while installing resources: ", err)
	}

	if len(want) != len(client.creates) {
		t.Fatalf("Unexpected creates: %s", fmt.Sprintf("(-got, +want): %s", cmp.Diff(client.creates, want)))
	}

	// reset created array
	client.creates = []unstructured.Unstructured{}

	want = []unstructured.Unstructured{serviceAccount, clusterRoleBinding, role,
		roleBinding, configMap, secret, hpa, service}

	err = i.EnsureNamespaceScopedResources()
	if err != nil {
		t.Fatal("Unexpected Error while installing resources: ", err)
	}

	if len(want) != len(client.creates) {
		t.Fatalf("Unexpected creates: %s", fmt.Sprintf("(-got, +want): %s", cmp.Diff(client.creates, want)))
	}

	// reset created array
	client.creates = []unstructured.Unstructured{}
	client.resourcesExist = false
	client.getErr = errors.NewNotFound(schema.GroupResource{
		Group:    "apps/v1",
		Resource: "Deployment",
	}, "test-deployment")

	err = i.EnsureDeploymentResources()
	assert.Error(t, err, v1alpha1.RECONCILE_AGAIN_ERR.Error())

	want = []unstructured.Unstructured{deployment}
	if len(want) != len(client.creates) {
		t.Fatalf("Unexpected creates: %s", fmt.Sprintf("(-got, +want): %s", cmp.Diff(client.creates, want)))
	}

	client.resourcesExist = true
	client.gets = []unstructured.Unstructured{deployment}
	err = i.EnsureDeploymentResources()
	if err != nil {
		t.Fatal("Unexpected Error while installing resources: ", err)
	}
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
)

func TestControllerReady(t *testing.T) {
	in := []unstructured.Unstructured{namespacedResource("apps/v1", "Deployment", "test", "ready-controller")}

	client := fake.New([]runtime.Object{readyControllerDeployment}...)
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	i := installer{
		Manifest: manifest,
	}

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

	i := installer{
		Manifest: manifest,
	}

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

	i := installer{
		Manifest: manifest,
	}

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

	i := installer{
		Manifest: manifest,
	}

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

	i := installer{
		Manifest: manifest,
	}

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

	i := installer{
		Manifest: manifest,
	}

	err = i.AllDeploymentsReady()
	if err == nil {
		t.Fatal("Expected Error but got nil ")
	}
}
