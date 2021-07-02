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
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestInstall(t *testing.T) {
	koPath := "testdata/kodata"
	os.Setenv(KoEnvKey, koPath)
	defer os.Unsetenv(KoEnvKey)

	// Resources in the manifest
	targetNamespace := "tekton-pipelines"
	wantdeployment := namespacedResource("apps/v1", "Deployment", "test", "test-deployment")
	wantrole := namespacedResource("rbac.authorization.k8s.io/v1", "Role", "test", "test-role")
	wantroleBinding := namespacedResource("rbac.authorization.k8s.io/v1", "RoleBinding", "test", "test-role-binding")
	wantclusterRole := clusterScopedResource("rbac.authorization.k8s.io/v1", "ClusterRole", "test-cluster-role")
	wantclusterRoleBinding := clusterScopedResource("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "test-cluster-role-binding")
	deployment := namespacedResource("apps/v1", "Deployment", "test", "test-deployment")
	role := namespacedResource("rbac.authorization.k8s.io/v1", "Role", "test", "test-role")
	roleBinding := namespacedResource("rbac.authorization.k8s.io/v1", "RoleBinding", "test", "test-role-binding")
	clusterRole := clusterScopedResource("rbac.authorization.k8s.io/v1", "ClusterRole", "test-cluster-role")
	clusterRoleBinding := clusterScopedResource("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "test-cluster-role-binding")
	//clusterTriggerBinding := clusterScopedResource("triggers.tekton.dev/v1alpha1", "ClusterTriggerBinding", "test-cluster-trigger-binding")
	//consoleCLIDownload := clusterScopedResource("console.openshift.io/v1", "ConsoleCLIDownload", "test-console-cli")

	// Deliberately mixing the order in the manifest.
	in := []unstructured.Unstructured{deployment, role, roleBinding, clusterRole, clusterRoleBinding}
	// Expect things to be applied in order.
	want := []unstructured.Unstructured{wantrole, wantclusterRole, wantroleBinding, wantclusterRoleBinding, wantdeployment}

	client := &fakeClient{}
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	instance := &v1alpha1.TektonPipeline{
		Spec: v1alpha1.TektonPipelineSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: targetNamespace,
			},
		},
	}
	if err := Install(context.TODO(), &manifest, instance); err != nil {
		t.Fatalf("Install() = %v, want no error", err)
	}

	if !cmp.Equal(client.creates, want) {
		t.Fatalf("Unexpected creates: %s", fmt.Sprintf("(-got, +want): %s", cmp.Diff(client.creates, want)))
	}

	condition := instance.Status.GetCondition(v1alpha1.InstallSucceeded)
	if condition == nil || condition.Status != corev1.ConditionTrue {
		t.Fatalf("InstallSucceeded = %v, want %v", condition, corev1.ConditionTrue)
	}
}

func TestInstallError(t *testing.T) {
	targetNamespace := "tekton-pipelines"
	koPath := "testdata/kodata"
	os.Setenv(KoEnvKey, koPath)
	defer os.Unsetenv(KoEnvKey)

	client := &fakeClient{err: errors.New("test")}
	manifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{
		namespacedResource("apps/v1", "Deployment", "test", "test-deployment"),
	}), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	instance := &v1alpha1.TektonPipeline{
		Spec: v1alpha1.TektonPipelineSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: targetNamespace,
			},
		},
	}
	if err := Install(context.TODO(), &manifest, instance); err == nil {
		t.Fatalf("Install() = nil, wanted an error")
	}

	condition := instance.Status.GetCondition(v1alpha1.InstallSucceeded)
	if condition == nil || condition.Status != corev1.ConditionFalse {
		t.Fatalf("InstallSucceeded = %v, want %v", condition, corev1.ConditionFalse)
	}

	addonManifest, err := mf.ManifestFrom(mf.Slice([]unstructured.Unstructured{
		clusterScopedResource("triggers.tekton.dev/v1alpha1", "ClusterTriggerBinding", "test-cluster-trigger-binding"),
	}), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	addonInstance := &v1alpha1.TektonAddon{
		Spec: v1alpha1.TektonAddonSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: targetNamespace,
			},
		},
	}
	if err := Install(context.TODO(), &addonManifest, addonInstance); err == nil {
		t.Fatalf("Install() = nil, wanted an error")
	}

	condition = addonInstance.Status.GetCondition(v1alpha1.InstallSucceeded)
	if condition == nil || condition.Status != corev1.ConditionFalse {
		t.Fatalf("InstallSucceeded = %v, want %v", condition, corev1.ConditionFalse)
	}
}

func TestUninstall(t *testing.T) {
	// Resources in the manifest
	deployment := namespacedResource("apps/v1", "Deployment", "test", "test-deployment")
	role := namespacedResource("rbac.authorization.k8s.io/v1", "Role", "test", "test-role")
	roleBinding := namespacedResource("rbac.authorization.k8s.io/v1", "RoleBinding", "test", "test-role-binding")
	clusterRole := clusterScopedResource("rbac.authorization.k8s.io/v1", "ClusterRole", "test-cluster-role")
	clusterRoleBinding := clusterScopedResource("rbac.authorization.k8s.io/v1", "ClusterRoleBinding", "test-cluster-role-binding")
	crd := clusterScopedResource("apiextensions.k8s.io/v1beta1", "CustomResourceDefinition", "test-crd")

	// Deliberately mixing the order in the manifest.
	in := []unstructured.Unstructured{crd, deployment, role, roleBinding, clusterRole, clusterRoleBinding}
	// Expect things to be deleted, non-rbac resources first and then in reversed order.
	want := []unstructured.Unstructured{crd, deployment, clusterRoleBinding, clusterRole, roleBinding, role}

	client := &fakeClient{resourcesExist: true}
	manifest, err := mf.ManifestFrom(mf.Slice(in), mf.UseClient(client))
	if err != nil {
		t.Fatalf("Failed to generate manifest: %v", err)
	}

	if err := Uninstall(context.TODO(), &manifest, nil); err != nil {
		t.Fatalf("Uninstall() = %v, want no error", err)
	}

	if !cmp.Equal(client.deletes, want) {
		t.Fatalf("Unexpected deletes: %s", cmp.Diff(client.deletes, want))
	}
}

type fakeClient struct {
	err            error
	resourcesExist bool
	creates        []unstructured.Unstructured
	deletes        []unstructured.Unstructured
}

func (f *fakeClient) Get(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
	var resource *unstructured.Unstructured
	if f.resourcesExist {
		resource = &unstructured.Unstructured{}
	}
	return resource, f.err
}

func (f *fakeClient) Delete(obj *unstructured.Unstructured, options ...mf.DeleteOption) error {
	f.deletes = append(f.deletes, *obj)
	return f.err
}

func (f *fakeClient) Create(obj *unstructured.Unstructured, options ...mf.ApplyOption) error {
	obj.SetAnnotations(nil) // Deleting the extra annotation. Irrelevant for the test.
	f.creates = append(f.creates, *obj)
	return f.err
}

func (f *fakeClient) Update(obj *unstructured.Unstructured, options ...mf.ApplyOption) error {
	return f.err
}
