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

package namespacesync

import (
	"context"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorfake "github.com/tektoncd/operator/pkg/client/clientset/versioned/fake"
	operatorinformers "github.com/tektoncd/operator/pkg/client/informers/externalversions"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func boolPtr(b bool) *bool { return &b }

// newTestReconciler builds a Reconciler backed by fake clients and pre-populates
// the informer stores. Namespaces listed here are reachable via r.nsLister.Get.
func newTestReconciler(t *testing.T, tc *v1alpha1.TektonConfig, namespaces []corev1.Namespace) (*Reconciler, *kubefake.Clientset) {
	t.Helper()

	kubeClient := kubefake.NewSimpleClientset()
	operatorClient := operatorfake.NewSimpleClientset(tc)

	kubeInformers := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	operatorInfs := operatorinformers.NewSharedInformerFactory(operatorClient, 0)

	tcInformer := operatorInfs.Operator().V1alpha1().TektonConfigs()
	nsInformer := kubeInformers.Core().V1().Namespaces()
	saInformer := kubeInformers.Core().V1().ServiceAccounts()

	assert.NilError(t, tcInformer.Informer().GetStore().Add(tc))
	for i := range namespaces {
		assert.NilError(t, nsInformer.Informer().GetStore().Add(&namespaces[i]))
	}

	return &Reconciler{
		kubeClient:         kubeClient,
		operatorClient:     operatorClient,
		nsLister:           nsInformer.Lister(),
		saLister:           saInformer.Lister().ServiceAccounts(""),
		tektonConfigLister: tcInformer.Lister(),
	}, kubeClient
}

func minimalTC(cfg *v1alpha1.NamespaceSyncConfig) *v1alpha1.TektonConfig {
	return &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha1.ConfigResourceName},
		Spec: v1alpha1.TektonConfigSpec{
			Platforms: v1alpha1.Platforms{
				OpenShift: v1alpha1.OpenShift{
					NamespaceSync: cfg,
				},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Reconcile top-level routing
// ---------------------------------------------------------------------------

func TestReconcile_TektonConfigNotFound(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	operatorClient := operatorfake.NewSimpleClientset() // no TektonConfig in store

	kubeInformers := kubeinformers.NewSharedInformerFactory(kubeClient, 0)
	operatorInfs := operatorinformers.NewSharedInformerFactory(operatorClient, 0)

	r := &Reconciler{
		kubeClient:         kubeClient,
		operatorClient:     operatorClient,
		nsLister:           kubeInformers.Core().V1().Namespaces().Lister(),
		saLister:           kubeInformers.Core().V1().ServiceAccounts().Lister().ServiceAccounts(""),
		tektonConfigLister: operatorInfs.Operator().V1alpha1().TektonConfigs().Lister(),
	}

	// TektonConfig lister cache is empty → should return nil without error.
	err := r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)
}

func TestReconcile_NamespaceSyncNil(t *testing.T) {
	tc := minimalTC(nil)
	r, _ := newTestReconciler(t, tc, nil)

	err := r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)
}

func TestReconcile_SystemNamespaceIgnored(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{CreatePipelineSA: boolPtr(true)})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "openshift-operators"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	err := r.Reconcile(context.Background(), "openshift-operators")
	assert.NilError(t, err)

	// pipeline SA must NOT be created in system namespaces.
	_, err = kubeClient.CoreV1().ServiceAccounts("openshift-operators").Get(context.Background(), pipelineSA, metav1.GetOptions{})
	assert.ErrorContains(t, err, "not found")
}

func TestReconcile_TerminatingNamespaceIgnored(t *testing.T) {
	now := metav1.Now()
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{CreatePipelineSA: boolPtr(true)})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns", DeletionTimestamp: &now}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	err := r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	_, err = kubeClient.CoreV1().ServiceAccounts("my-ns").Get(context.Background(), pipelineSA, metav1.GetOptions{})
	assert.ErrorContains(t, err, "not found")
}

// ---------------------------------------------------------------------------
// ensurePipelineSA
// ---------------------------------------------------------------------------

func TestEnsurePipelineSA_CreatesWhenAbsent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{CreatePipelineSA: boolPtr(true)})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	err := r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	sa, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Get(context.Background(), pipelineSA, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, pipelineSA, sa.Name)
	assert.Equal(t, 1, len(sa.OwnerReferences))
	assert.Equal(t, v1alpha1.ConfigResourceName, sa.OwnerReferences[0].Name)
}

func TestEnsurePipelineSA_SetsOwnerRefOnExistingSA(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{CreatePipelineSA: boolPtr(true)})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	// pre-create a pipeline SA with no owner reference
	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineSA, Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	err = r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	sa, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Get(context.Background(), pipelineSA, metav1.GetOptions{})
	assert.NilError(t, err)
	// owner ref should be set on the existing SA
	assert.Equal(t, 1, len(sa.OwnerReferences))
	assert.Equal(t, v1alpha1.ConfigResourceName, sa.OwnerReferences[0].Name)
}

func TestEnsurePipelineSA_IdempotentWhenOwnerRefPresent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{CreatePipelineSA: boolPtr(true)})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	ownerRef := tektonConfigOwnerRef(tc)
	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pipelineSA,
			Namespace:       "my-ns",
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	// Reconcile twice — should be a no-op on the second call.
	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))
	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))

	sa, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Get(context.Background(), pipelineSA, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, 1, len(sa.OwnerReferences))
}

func TestEnsurePipelineSA_SkippedWhenDisabled(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{CreatePipelineSA: boolPtr(false)})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	err := r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	_, err = kubeClient.CoreV1().ServiceAccounts("my-ns").Get(context.Background(), pipelineSA, metav1.GetOptions{})
	assert.ErrorContains(t, err, "not found")
}

// ---------------------------------------------------------------------------
// ensureEditRoleBinding
// ---------------------------------------------------------------------------

func TestEnsureEditRoleBinding_CreatesWhenAbsent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA:      boolPtr(false),
		CreateEditRoleBinding: boolPtr(true),
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	// The reconciler verifies that the edit ClusterRole exists before creating the binding.
	_, err := kubeClient.RbacV1().ClusterRoles().Create(context.Background(), &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: editClusterRole},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	err = r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	rb, err := kubeClient.RbacV1().RoleBindings("my-ns").Get(context.Background(), PipelineRoleBinding, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, editClusterRole, rb.RoleRef.Name)
	assert.Equal(t, pipelineSA, rb.Subjects[0].Name)
}

func TestEnsureEditRoleBinding_IdempotentWhenPresent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA:      boolPtr(false),
		CreateEditRoleBinding: boolPtr(true),
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	_, err := kubeClient.RbacV1().ClusterRoles().Create(context.Background(), &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: editClusterRole},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)
	_, err = kubeClient.RbacV1().RoleBindings("my-ns").Create(context.Background(), &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: PipelineRoleBinding, Namespace: "my-ns"},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: editClusterRole},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	// Second reconcile — must not attempt to create again.
	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))
	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))
}

func TestRemoveEditRoleBinding_DeletesWhenPresent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA:      boolPtr(false),
		CreateEditRoleBinding: boolPtr(false),
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	// pre-create the RoleBinding — reconcile must delete it.
	_, err := kubeClient.RbacV1().RoleBindings("my-ns").Create(context.Background(), &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: PipelineRoleBinding, Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	err = r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	_, err = kubeClient.RbacV1().RoleBindings("my-ns").Get(context.Background(), PipelineRoleBinding, metav1.GetOptions{})
	assert.ErrorContains(t, err, "not found")
}

func TestRemoveEditRoleBinding_NoopWhenAbsent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA:      boolPtr(false),
		CreateEditRoleBinding: boolPtr(false),
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, _ := newTestReconciler(t, tc, []corev1.Namespace{ns})

	// No RoleBinding pre-exists — should not error.
	err := r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)
}

// ---------------------------------------------------------------------------
// ensureSecretBindings
// ---------------------------------------------------------------------------

func TestEnsureSecretBindings_ByName_BindsWhenSecretExists(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA: boolPtr(false),
		SecretBindings:   []v1alpha1.SecretBinding{{SecretName: "quay-robot"}},
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineSA, Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)
	_, err = kubeClient.CoreV1().Secrets("my-ns").Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "quay-robot", Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	err = r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	sa, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Get(context.Background(), pipelineSA, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, 1, len(sa.ImagePullSecrets))
	assert.Equal(t, "quay-robot", sa.ImagePullSecrets[0].Name)
	assert.Equal(t, 1, len(sa.Secrets))
	assert.Equal(t, "quay-robot", sa.Secrets[0].Name)
}

func TestEnsureSecretBindings_ByName_SkipsWhenSecretAbsent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA: boolPtr(false),
		SecretBindings:   []v1alpha1.SecretBinding{{SecretName: "quay-robot"}},
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	// pipeline SA exists, but the named secret does not
	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineSA, Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	err = r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	// SA must have no bindings — no infinite loop / error.
	sa, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Get(context.Background(), pipelineSA, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, 0, len(sa.ImagePullSecrets))
	assert.Equal(t, 0, len(sa.Secrets))
}

func TestEnsureSecretBindings_Idempotent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA: boolPtr(false),
		SecretBindings:   []v1alpha1.SecretBinding{{SecretName: "quay-robot"}},
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	// SA already has the binding.
	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta:       metav1.ObjectMeta{Name: pipelineSA, Namespace: "my-ns"},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "quay-robot"}},
		Secrets:          []corev1.ObjectReference{{Name: "quay-robot"}},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)
	_, err = kubeClient.CoreV1().Secrets("my-ns").Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "quay-robot", Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))
	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))

	sa, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Get(context.Background(), pipelineSA, metav1.GetOptions{})
	assert.NilError(t, err)
	// must not have doubled up
	assert.Equal(t, 1, len(sa.ImagePullSecrets))
	assert.Equal(t, 1, len(sa.Secrets))
}

func TestEnsureSecretBindings_ByLabel(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA: boolPtr(false),
		SecretBindings: []v1alpha1.SecretBinding{{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"quay.io/robot-token": "true"},
			},
		}},
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineSA, Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)
	_, err = kubeClient.CoreV1().Secrets("my-ns").Create(context.Background(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "quay-robot-token",
			Namespace: "my-ns",
			Labels:    map[string]string{"quay.io/robot-token": "true"},
		},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	err = r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	sa, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Get(context.Background(), pipelineSA, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, 1, len(sa.ImagePullSecrets))
	assert.Equal(t, "quay-robot-token", sa.ImagePullSecrets[0].Name)
}

func TestEnsureSecretBindings_SkipsWhenSAAbsent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA: boolPtr(false), // SA won't be created by this reconcile
		SecretBindings:   []v1alpha1.SecretBinding{{SecretName: "quay-robot"}},
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, _ := newTestReconciler(t, tc, []corev1.Namespace{ns})

	// pipeline SA does not exist — ensureSecretBindings should be a no-op.
	err := r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)
}

// ---------------------------------------------------------------------------
// bindSecretToSA unit tests
// ---------------------------------------------------------------------------

func TestBindSecretToSA_AddsToEmptySA(t *testing.T) {
	sa := &corev1.ServiceAccount{}
	changed := bindSecretToSA(sa, "my-secret")
	assert.Equal(t, true, changed)
	assert.Equal(t, 1, len(sa.ImagePullSecrets))
	assert.Equal(t, "my-secret", sa.ImagePullSecrets[0].Name)
	assert.Equal(t, 1, len(sa.Secrets))
	assert.Equal(t, "my-secret", sa.Secrets[0].Name)
}

func TestBindSecretToSA_Idempotent(t *testing.T) {
	sa := &corev1.ServiceAccount{
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "my-secret"}},
		Secrets:          []corev1.ObjectReference{{Name: "my-secret"}},
	}
	changed := bindSecretToSA(sa, "my-secret")
	assert.Equal(t, false, changed)
	assert.Equal(t, 1, len(sa.ImagePullSecrets))
	assert.Equal(t, 1, len(sa.Secrets))
}

func TestBindSecretToSA_AddsOnlyImagePullWhenMountPresent(t *testing.T) {
	sa := &corev1.ServiceAccount{
		Secrets: []corev1.ObjectReference{{Name: "my-secret"}},
	}
	changed := bindSecretToSA(sa, "my-secret")
	assert.Equal(t, true, changed) // imagePullSecret was missing
	assert.Equal(t, 1, len(sa.ImagePullSecrets))
	assert.Equal(t, 1, len(sa.Secrets))
}

// ---------------------------------------------------------------------------
// ensureCABundles
// ---------------------------------------------------------------------------

func TestEnsureCABundles_CreatesWhenAbsent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA: boolPtr(false),
		CreateCABundles:  boolPtr(true),
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	err := r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	_, err = kubeClient.CoreV1().ConfigMaps("my-ns").Get(context.Background(), trustedCABundleConfigMap, metav1.GetOptions{})
	assert.NilError(t, err)

	_, err = kubeClient.CoreV1().ConfigMaps("my-ns").Get(context.Background(), serviceCABundleConfigMap, metav1.GetOptions{})
	assert.NilError(t, err)
}

func TestEnsureCABundles_StripsOwnerRefFromExisting(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA: boolPtr(false),
		CreateCABundles:  boolPtr(true),
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	// Pre-create CMs with an owner reference.
	for _, name := range []string{trustedCABundleConfigMap, serviceCABundleConfigMap} {
		_, err := kubeClient.CoreV1().ConfigMaps("my-ns").Create(context.Background(), &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: "my-ns",
				OwnerReferences: []metav1.OwnerReference{
					{Name: "some-owner", APIVersion: "v1", Kind: "Thing"},
				},
			},
		}, metav1.CreateOptions{})
		assert.NilError(t, err)
	}

	err := r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	cm, err := kubeClient.CoreV1().ConfigMaps("my-ns").Get(context.Background(), trustedCABundleConfigMap, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, 0, len(cm.OwnerReferences))
}

func TestEnsureCABundles_Idempotent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA: boolPtr(false),
		CreateCABundles:  boolPtr(true),
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))
	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))

	_, err := kubeClient.CoreV1().ConfigMaps("my-ns").Get(context.Background(), trustedCABundleConfigMap, metav1.GetOptions{})
	assert.NilError(t, err)
}

// ---------------------------------------------------------------------------
// ensureSCCRoleBinding
// ---------------------------------------------------------------------------

func TestEnsureSCCRoleBinding_SkipsWhenSAAbsent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA:     boolPtr(false),
		CreateSCCRoleBinding: boolPtr(true),
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})
	// No security client needed — SA is absent so we return early.
	r.securityClientSet = nil

	err := r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	_, err = kubeClient.RbacV1().RoleBindings("my-ns").Get(context.Background(), pipelinesSCCRoleBinding, metav1.GetOptions{})
	assert.ErrorContains(t, err, "not found")
}

func TestEnsureSCCRoleBinding_CreatesWithClusterRoleWhenNoAnnotation(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA:     boolPtr(false),
		CreateSCCRoleBinding: boolPtr(true),
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})
	r.securityClientSet = nil // no SCC annotation → security client not needed

	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineSA, Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	err = r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	rb, err := kubeClient.RbacV1().RoleBindings("my-ns").Get(context.Background(), pipelinesSCCRoleBinding, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, "ClusterRole", rb.RoleRef.Kind)
	assert.Equal(t, pipelinesSCCClusterRole, rb.RoleRef.Name)
	assert.Equal(t, pipelineSA, rb.Subjects[0].Name)
}

func TestEnsureSCCRoleBinding_Idempotent(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA:     boolPtr(false),
		CreateSCCRoleBinding: boolPtr(true),
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})
	r.securityClientSet = nil

	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineSA, Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))
	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))

	// Exactly one RoleBinding.
	rbs, err := kubeClient.RbacV1().RoleBindings("my-ns").List(context.Background(), metav1.ListOptions{})
	assert.NilError(t, err)
	assert.Equal(t, 1, len(rbs.Items))
}

func TestEnsureSCCRoleBinding_DeletesLeftoverRoleWhenNoAnnotation(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA:     boolPtr(false),
		CreateSCCRoleBinding: boolPtr(true),
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})
	r.securityClientSet = nil

	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineSA, Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)
	// Pre-create a leftover namespace-scoped role (from a previous annotation).
	_, err = kubeClient.RbacV1().Roles("my-ns").Create(context.Background(), &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Name: pipelinesSCCRole, Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	err = r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	// Role must be deleted.
	_, err = kubeClient.RbacV1().Roles("my-ns").Get(context.Background(), pipelinesSCCRole, metav1.GetOptions{})
	assert.ErrorContains(t, err, "not found")
}

// ---------------------------------------------------------------------------
// Stale secret removal
// ---------------------------------------------------------------------------

func TestEnsureSecretBindings_RemovesStaleNamedBinding(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA: boolPtr(false),
		SecretBindings:   []v1alpha1.SecretBinding{{SecretName: "quay-robot"}},
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	// SA already bound to a secret that no longer exists.
	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta:       metav1.ObjectMeta{Name: pipelineSA, Namespace: "my-ns"},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "quay-robot"}},
		Secrets:          []corev1.ObjectReference{{Name: "quay-robot"}},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)
	// Secret does NOT exist → should be removed from SA.

	err = r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	sa, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Get(context.Background(), pipelineSA, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, 0, len(sa.ImagePullSecrets))
	assert.Equal(t, 0, len(sa.Secrets))
}

func TestEnsureSecretBindings_KeepsUnmanagedSecrets(t *testing.T) {
	tc := minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA: boolPtr(false),
		SecretBindings:   []v1alpha1.SecretBinding{{SecretName: "quay-robot"}},
	})
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, tc, []corev1.Namespace{ns})

	// SA has a user-added secret that is NOT in any binding.
	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta:       metav1.ObjectMeta{Name: pipelineSA, Namespace: "my-ns"},
		ImagePullSecrets: []corev1.LocalObjectReference{{Name: "user-added-secret"}},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	err = r.Reconcile(context.Background(), "my-ns")
	assert.NilError(t, err)

	sa, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Get(context.Background(), pipelineSA, metav1.GetOptions{})
	assert.NilError(t, err)
	// user-added-secret must be preserved — we do not own it.
	assert.Equal(t, 1, len(sa.ImagePullSecrets))
	assert.Equal(t, "user-added-secret", sa.ImagePullSecrets[0].Name)
}

// ---------------------------------------------------------------------------
// ClusterInterceptors ClusterRoleBinding tests
// ---------------------------------------------------------------------------

func TestClusterInterceptors_CreatesCRBWhenSAExists(t *testing.T) {
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, minimalTC(&v1alpha1.NamespaceSyncConfig{}), []corev1.Namespace{ns})

	// Pipeline SA exists — reconcile should add it to the ClusterRoleBinding.
	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineSA, Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))

	crb, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), clusterInterceptorsClusterRole, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, 1, len(crb.Subjects))
	assert.Equal(t, "my-ns", crb.Subjects[0].Namespace)
	assert.Equal(t, pipelineSA, crb.Subjects[0].Name)
}

func TestClusterInterceptors_NoCRBWhenSAAbsent(t *testing.T) {
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, minimalTC(&v1alpha1.NamespaceSyncConfig{
		CreatePipelineSA: boolPtr(false),
	}), []corev1.Namespace{ns})

	// No SA — ClusterRoleBinding must not be created.
	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))

	_, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), clusterInterceptorsClusterRole, metav1.GetOptions{})
	assert.Assert(t, errors.IsNotFound(err), "expected ClusterRoleBinding to be absent, got: %v", err)
}

func TestClusterInterceptors_Idempotent(t *testing.T) {
	ns := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "my-ns"}}
	r, kubeClient := newTestReconciler(t, minimalTC(&v1alpha1.NamespaceSyncConfig{}), []corev1.Namespace{ns})

	_, err := kubeClient.CoreV1().ServiceAccounts("my-ns").Create(context.Background(), &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineSA, Namespace: "my-ns"},
	}, metav1.CreateOptions{})
	assert.NilError(t, err)

	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))
	assert.NilError(t, r.Reconcile(context.Background(), "my-ns"))

	crb, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), clusterInterceptorsClusterRole, metav1.GetOptions{})
	assert.NilError(t, err)
	// Must not have duplicate subjects.
	count := 0
	for _, s := range crb.Subjects {
		if s.Namespace == "my-ns" && s.Name == pipelineSA {
			count++
		}
	}
	assert.Equal(t, 1, count)
}

func TestClusterInterceptors_AddsMultipleNamespaces(t *testing.T) {
	nsA := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-a"}}
	nsB := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-b"}}
	r, kubeClient := newTestReconciler(t, minimalTC(&v1alpha1.NamespaceSyncConfig{}), []corev1.Namespace{nsA, nsB})

	for _, nsName := range []string{"ns-a", "ns-b"} {
		_, err := kubeClient.CoreV1().ServiceAccounts(nsName).Create(context.Background(), &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: pipelineSA, Namespace: nsName},
		}, metav1.CreateOptions{})
		assert.NilError(t, err)
		assert.NilError(t, r.Reconcile(context.Background(), nsName))
	}

	crb, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), clusterInterceptorsClusterRole, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, 2, len(crb.Subjects))
}

func TestClusterInterceptors_RemovesSubjectWhenSADeleted(t *testing.T) {
	nsA := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-a"}}
	nsB := corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "ns-b"}}
	r, kubeClient := newTestReconciler(t, minimalTC(&v1alpha1.NamespaceSyncConfig{}), []corev1.Namespace{nsA, nsB})

	// Create SAs in both namespaces and reconcile.
	for _, nsName := range []string{"ns-a", "ns-b"} {
		_, err := kubeClient.CoreV1().ServiceAccounts(nsName).Create(context.Background(), &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Name: pipelineSA, Namespace: nsName},
		}, metav1.CreateOptions{})
		assert.NilError(t, err)
		assert.NilError(t, r.Reconcile(context.Background(), nsName))
	}

	// Delete ns-a's pipeline SA then reconcile ns-a — its subject must be removed.
	assert.NilError(t, kubeClient.CoreV1().ServiceAccounts("ns-a").Delete(context.Background(), pipelineSA, metav1.DeleteOptions{}))
	assert.NilError(t, r.Reconcile(context.Background(), "ns-a"))

	crb, err := kubeClient.RbacV1().ClusterRoleBindings().Get(context.Background(), clusterInterceptorsClusterRole, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, 1, len(crb.Subjects))
	assert.Equal(t, "ns-b", crb.Subjects[0].Namespace)
}
