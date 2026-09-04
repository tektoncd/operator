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

// Package namespacesync implements the NamespaceSyncController, a watch-based
// controller that ensures Tekton-required resources (pipeline SA, CA bundles,
// SCC RoleBinding, edit RoleBinding, and registry secret bindings) are present
// and up to date in every user namespace on OpenShift.
package namespacesync

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	security "github.com/openshift/client-go/security/clientset/versioned"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	tektonConfiglister "github.com/tektoncd/operator/pkg/client/listers/operator/v1alpha1"
	pkgcommon "github.com/tektoncd/operator/pkg/common"
	reconcilerCommon "github.com/tektoncd/operator/pkg/reconciler/common"
	openshiftpkg "github.com/tektoncd/operator/pkg/reconciler/openshift"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	corelisterv1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/retry"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
)

const (
	pipelineSA              = "pipeline"
	pipelinesSCCRole        = "pipelines-scc-role"
	pipelinesSCCClusterRole = "pipelines-scc-clusterrole"
	pipelinesSCCRoleBinding = "pipelines-scc-rolebinding"
	// PipelineRoleBinding is the name of the edit RoleBinding created in each
	// user namespace so that the pipeline SA has edit access. Exported for use
	// in e2e tests.
	PipelineRoleBinding            = "openshift-pipelines-edit"
	editClusterRole                = "edit"
	serviceCABundleConfigMap       = "config-service-cabundle"
	trustedCABundleConfigMap       = "config-trusted-cabundle"
	clusterInterceptorsClusterRole = "openshift-pipelines-clusterinterceptors"
	// managedSecretBindingsAnnotation stores the comma-separated set of secret
	// names that this controller currently manages on the pipeline SA (derived
	// from spec.platforms.openshift.namespaceSync.secretBindings at the time of
	// the last reconcile). It lets a later reconcile detect and remove secret
	// references whose binding rule was changed or removed entirely, since
	// bindings no longer present in spec cannot otherwise be told apart from
	// secrets a user bound manually.
	managedSecretBindingsAnnotation = "operator.tekton.dev/namespacesync-managed-secrets"
)

var nsIgnoreRegex = regexp.MustCompile(reconcilerCommon.NamespaceIgnorePattern)

// Reconciler reconciles a namespace name as its work unit. It reads the
// current NamespaceSyncConfig from TektonConfig and ensures all declared
// resources exist and are correct in the given namespace.
type Reconciler struct {
	reconciler.LeaderAwareFuncs

	kubeClient         kubernetes.Interface
	operatorClient     clientset.Interface
	securityClientSet  security.Interface
	nsLister           corelisterv1.NamespaceLister
	saLister           corelisterv1.ServiceAccountNamespaceLister
	tektonConfigLister tektonConfiglister.TektonConfigLister
}

var _ interface {
	Reconcile(context.Context, string) error
} = (*Reconciler)(nil)

// Reconcile is the main entry point — key is the namespace name.
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	tc, err := r.tektonConfigLister.Get(v1alpha1.ConfigResourceName)
	if errors.IsNotFound(err) {
		logger.Debug("TektonConfig not found, skipping namespace sync")
		return nil
	}
	if err != nil {
		return err
	}

	tc = tc.DeepCopy()
	tc.SetDefaults(ctx)

	cfg := tc.Spec.Platforms.OpenShift.NamespaceSync
	if cfg == nil {
		logger.Debug("NamespaceSync config absent, skipping")
		return nil
	}

	ns, err := r.nsLister.Get(key)
	if errors.IsNotFound(err) {
		// Namespace has been deleted. Namespace-scoped resources (pipeline SA,
		// RoleBindings, ConfigMaps) are garbage collected by Kubernetes along
		// with the namespace, but the cluster-scoped ClusterInterceptors
		// ClusterRoleBinding is not — remove this namespace's subject from it
		// explicitly so deleted namespaces don't accumulate stale entries.
		return r.ensureClusterInterceptorsSubject(ctx, key, tc)
	}
	if err != nil {
		return err
	}

	if shouldIgnoreNamespace(ns) {
		logger.Debugf("Ignoring system/terminating namespace: %s", key)
		return nil
	}

	if !namespaceMatchesSelector(ns, cfg) {
		logger.Debugf("Namespace %s excluded by namespaceSelector, skipping", key)
		return nil
	}

	return r.reconcileNamespace(ctx, ns, tc, cfg)
}

func (r *Reconciler) reconcileNamespace(ctx context.Context, ns *corev1.Namespace, tc *v1alpha1.TektonConfig, cfg *v1alpha1.NamespaceSyncConfig) error {
	logger := logging.FromContext(ctx)

	if cfg.CreatePipelineSA != nil && *cfg.CreatePipelineSA {
		if err := r.ensurePipelineSA(ctx, ns, tc); err != nil {
			logger.Errorf("failed to ensure pipeline SA in %s: %v", ns.Name, err)
			return err
		}
	}

	// Keep the cluster-wide ClusterInterceptors ClusterRoleBinding up to date.
	// This is an incremental patch: add/remove just this namespace's pipeline SA
	// subject, rather than rebuilding the full subject list on every reconcile.
	if err := r.ensureClusterInterceptorsSubject(ctx, ns.Name, tc); err != nil {
		logger.Errorf("failed to sync ClusterInterceptors subject for %s: %v", ns.Name, err)
		return err
	}

	if cfg.CreateSCCRoleBinding != nil && *cfg.CreateSCCRoleBinding {
		if err := r.ensureSCCRoleBinding(ctx, ns, tc); err != nil {
			logger.Errorf("failed to ensure SCC RoleBinding in %s: %v", ns.Name, err)
			return err
		}
	}

	if cfg.CreateEditRoleBinding != nil && *cfg.CreateEditRoleBinding {
		if err := r.ensureEditRoleBinding(ctx, ns, tc); err != nil {
			logger.Errorf("failed to ensure edit RoleBinding in %s: %v", ns.Name, err)
			return err
		}
	} else {
		if err := r.removeEditRoleBindingIfPresent(ctx, ns.Name); err != nil {
			logger.Errorf("failed to remove edit RoleBinding from %s: %v", ns.Name, err)
			return err
		}
	}

	if cfg.CreateCABundles != nil && *cfg.CreateCABundles {
		if err := r.ensureCABundles(ctx, ns); err != nil {
			logger.Errorf("failed to ensure CA bundles in %s: %v", ns.Name, err)
			return err
		}
	}

	// Always run this, even when SecretBindings is empty: only ensureSecretBindings
	// knows (via managedSecretBindingsAnnotation) which secrets it previously bound,
	// so it is also the only place that can clean up references left behind after
	// spec.platforms.openshift.namespaceSync.secretBindings is changed or cleared.
	if err := r.ensureSecretBindings(ctx, ns, cfg.SecretBindings); err != nil {
		logger.Errorf("failed to ensure secret bindings in %s: %v", ns.Name, err)
		return err
	}

	return nil
}

// ---------------------------------------------------------------------------
// Pipeline ServiceAccount
// ---------------------------------------------------------------------------

// ensurePipelineSA creates the pipeline ServiceAccount if absent, or updates
// the owner reference if it already exists.
func (r *Reconciler) ensurePipelineSA(ctx context.Context, ns *corev1.Namespace, tc *v1alpha1.TektonConfig) error {
	logger := logging.FromContext(ctx)
	saClient := r.kubeClient.CoreV1().ServiceAccounts(ns.Name)

	sa, err := saClient.Get(ctx, pipelineSA, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		logger.Infof("Creating pipeline SA in namespace %s", ns.Name)
		ownerRef := tektonConfigOwnerRef(tc)
		_, err = saClient.Create(ctx, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:            pipelineSA,
				Namespace:       ns.Name,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
		}, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	ownerRef := tektonConfigOwnerRef(tc)
	if !hasOwnerReference(sa.OwnerReferences, ownerRef) {
		sa.OwnerReferences = []metav1.OwnerReference{ownerRef}
		_, err = saClient.Update(ctx, sa, metav1.UpdateOptions{})
		return err
	}
	return nil
}

// ---------------------------------------------------------------------------
// SCC RoleBinding
// ---------------------------------------------------------------------------

// ensureSCCRoleBinding creates or updates the pipelines-scc-rolebinding in the
// namespace, binding the pipeline SA to the appropriate SCC Role or ClusterRole.
// If the namespace carries the operator.tekton.dev/scc annotation, a
// namespace-scoped Role is created for that specific SCC; otherwise the cluster-
// wide pipelines-scc-clusterrole is used.
func (r *Reconciler) ensureSCCRoleBinding(ctx context.Context, ns *corev1.Namespace, tc *v1alpha1.TektonConfig) error {
	logger := logging.FromContext(ctx)

	// The RoleBinding subject is the pipeline SA. If it does not exist yet
	// the SA watch will re-enqueue this namespace once it is created.
	sa, err := r.kubeClient.CoreV1().ServiceAccounts(ns.Name).Get(ctx, pipelineSA, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		logger.Debugf("pipeline SA not yet present in %s, deferring SCC RoleBinding", ns.Name)
		return nil
	}
	if err != nil {
		return err
	}

	nsSCC := ns.Annotations[openshiftpkg.NamespaceSCCAnnotation]

	if nsSCC == "" {
		// No custom SCC annotation: clean up any leftover namespace-scoped Role
		// (left over from a previous annotation) and use the ClusterRole.
		if err := r.deleteRoleIfPresent(ctx, ns.Name, pipelinesSCCRole); err != nil {
			return err
		}
	} else {
		// A specific SCC has been requested for this namespace.
		logger.Infof("Namespace %s requests SCC %s", ns.Name, nsSCC)

		// Verify the SCC exists on the cluster.
		if err := pkgcommon.VerifySCCExists(ctx, nsSCC, r.securityClientSet); err != nil {
			logger.Errorf("SCC %s not found: %v", nsSCC, err)
			if evtErr := r.createSCCFailureEvent(ctx, ns.Name, nsSCC, tc); evtErr != nil {
				logger.Errorf("Failed to create SCC failure event in %s: %v", ns.Name, evtErr)
			}
			return err
		}

		// Enforce the maxAllowed SCC policy when configured.
		if tc.Spec.Platforms.OpenShift.SCC != nil && tc.Spec.Platforms.OpenShift.SCC.MaxAllowed != "" {
			maxAllowed := tc.Spec.Platforms.OpenShift.SCC.MaxAllowed
			list, err := pkgcommon.GetSCCRestrictiveList(ctx, r.securityClientSet)
			if err != nil {
				return err
			}
			ok, err := pkgcommon.SCCAMoreRestrictiveThanB(list, nsSCC, maxAllowed)
			if err != nil {
				return err
			}
			if !ok {
				return fmt.Errorf("namespace %s requested SCC %s which is less restrictive than maxAllowed %s", ns.Name, nsSCC, maxAllowed)
			}
		}

		// Ensure the namespace-scoped Role for this SCC.
		if err := r.ensureSCCRoleInNamespace(ctx, ns.Name, nsSCC, tc); err != nil {
			return err
		}
	}

	roleRef := r.getSCCRoleRef(ns)
	return r.ensurePipelinesSCCRoleBinding(ctx, sa, roleRef, tc)
}

// getSCCRoleRef returns a Role ref pointing at the namespace-scoped pipelines-scc-role
// when a custom SCC is requested, or the cluster-wide pipelines-scc-clusterrole otherwise.
func (r *Reconciler) getSCCRoleRef(ns *corev1.Namespace) *rbacv1.RoleRef {
	if ns.Annotations[openshiftpkg.NamespaceSCCAnnotation] != "" {
		return &rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     pipelinesSCCRole,
		}
	}
	return &rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     pipelinesSCCClusterRole,
	}
}

// ensureSCCRoleInNamespace creates or updates the namespace-scoped pipelines-scc-role
// that grants the `use` verb on the requested SCC.
func (r *Reconciler) ensureSCCRoleInNamespace(ctx context.Context, nsName, scc string, tc *v1alpha1.TektonConfig) error {
	ownerRef := tektonConfigOwnerRef(tc)
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pipelinesSCCRole,
			Namespace:       nsName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups:     []string{"security.openshift.io"},
			Resources:     []string{"securitycontextconstraints"},
			ResourceNames: []string{scc},
			Verbs:         []string{"use"},
		}},
	}

	rbacClient := r.kubeClient.RbacV1()
	_, err := rbacClient.Roles(nsName).Get(ctx, pipelinesSCCRole, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = rbacClient.Roles(nsName).Create(ctx, role, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}
	_, err = rbacClient.Roles(nsName).Update(ctx, role, metav1.UpdateOptions{})
	return err
}

// ensurePipelinesSCCRoleBinding creates or updates the pipelines-scc-rolebinding
// that binds the pipeline SA to the given role ref.
func (r *Reconciler) ensurePipelinesSCCRoleBinding(ctx context.Context, sa *corev1.ServiceAccount, roleRef *rbacv1.RoleRef, tc *v1alpha1.TektonConfig) error {
	logger := logging.FromContext(ctx)
	rbacClient := r.kubeClient.RbacV1()
	ownerRef := tektonConfigOwnerRef(tc)

	existing, err := rbacClient.RoleBindings(sa.Namespace).Get(ctx, pipelinesSCCRoleBinding, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		logger.Infof("Creating %s in namespace %s", pipelinesSCCRoleBinding, sa.Namespace)
		_, err = rbacClient.RoleBindings(sa.Namespace).Create(ctx, &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:            pipelinesSCCRoleBinding,
				Namespace:       sa.Namespace,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			RoleRef:  *roleRef,
			Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}},
		}, metav1.CreateOptions{})
		return err
	}
	if err != nil {
		return err
	}

	// RoleRef is immutable — delete and recreate if it changed.
	if existing.RoleRef.Kind != roleRef.Kind || existing.RoleRef.Name != roleRef.Name {
		logger.Infof("RoleRef changed in %s/%s, recreating", sa.Namespace, pipelinesSCCRoleBinding)
		if err := rbacClient.RoleBindings(sa.Namespace).Delete(ctx, pipelinesSCCRoleBinding, metav1.DeleteOptions{}); err != nil {
			return err
		}
		_, err = rbacClient.RoleBindings(sa.Namespace).Create(ctx, &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:            pipelinesSCCRoleBinding,
				Namespace:       sa.Namespace,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			RoleRef:  *roleRef,
			Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}},
		}, metav1.CreateOptions{})
		return err
	}

	// Ensure subject is present.
	subject := rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}
	if hasSubject(existing.Subjects, subject) {
		return nil
	}
	existing.Subjects = append(existing.Subjects, subject)
	_, err = rbacClient.RoleBindings(sa.Namespace).Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

// deleteRoleIfPresent deletes a namespace-scoped Role, ignoring not-found errors.
func (r *Reconciler) deleteRoleIfPresent(ctx context.Context, nsName, roleName string) error {
	err := r.kubeClient.RbacV1().Roles(nsName).Delete(ctx, roleName, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// createSCCFailureEvent records a Warning event in the namespace when the
// requested SCC is not found on the cluster.
func (r *Reconciler) createSCCFailureEvent(ctx context.Context, nsName, scc string, tc *v1alpha1.TektonConfig) error {
	ownerRef := tektonConfigOwnerRef(tc)
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    "pipelines-scc-failure-",
			Namespace:       nsName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		EventTime:           metav1.NewMicroTime(time.Now()),
		Reason:              "RequestedSCCNotFound",
		Type:                "Warning",
		Action:              "SCCNotUpdated",
		Message:             fmt.Sprintf("SCC '%s' requested in annotation '%s' not found, SCC not updated in the namespace", scc, openshiftpkg.NamespaceSCCAnnotation),
		ReportingController: "openshift-pipelines-operator",
		ReportingInstance:   tc.Name,
		InvolvedObject: corev1.ObjectReference{
			Kind:       "Namespace",
			Name:       nsName,
			APIVersion: "v1",
			Namespace:  nsName,
		},
	}
	_, err := r.kubeClient.CoreV1().Events(nsName).Create(ctx, event, metav1.CreateOptions{})
	return err
}

// ---------------------------------------------------------------------------
// Edit RoleBinding
// ---------------------------------------------------------------------------

// ensureEditRoleBinding creates the openshift-pipelines-edit RoleBinding binding
// the pipeline SA to the built-in edit ClusterRole, or corrects it if it already
// exists but has drifted (missing owner reference, wrong RoleRef, or missing
// subject).
func (r *Reconciler) ensureEditRoleBinding(ctx context.Context, ns *corev1.Namespace, tc *v1alpha1.TektonConfig) error {
	logger := logging.FromContext(ctx)
	rbClient := r.kubeClient.RbacV1().RoleBindings(ns.Name)
	ownerRef := tektonConfigOwnerRef(tc)

	wantRoleRef := rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     editClusterRole,
	}
	wantSubject := rbacv1.Subject{
		Kind:      rbacv1.ServiceAccountKind,
		Name:      pipelineSA,
		Namespace: ns.Name,
	}

	existing, err := rbClient.Get(ctx, PipelineRoleBinding, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	if errors.IsNotFound(err) {
		// Verify the edit ClusterRole exists before creating the binding.
		if _, err := r.kubeClient.RbacV1().ClusterRoles().Get(ctx, editClusterRole, metav1.GetOptions{}); err != nil {
			return err
		}

		logger.Infof("Creating edit RoleBinding in namespace %s", ns.Name)
		_, err = rbClient.Create(ctx, &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:            PipelineRoleBinding,
				Namespace:       ns.Name,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			RoleRef:  wantRoleRef,
			Subjects: []rbacv1.Subject{wantSubject},
		}, metav1.CreateOptions{})
		return err
	}

	// RoleRef is immutable — delete and recreate if it changed.
	if existing.RoleRef.Kind != wantRoleRef.Kind || existing.RoleRef.Name != wantRoleRef.Name {
		logger.Infof("RoleRef changed in %s/%s, recreating", ns.Name, PipelineRoleBinding)
		if err := rbClient.Delete(ctx, PipelineRoleBinding, metav1.DeleteOptions{}); err != nil && !errors.IsNotFound(err) {
			return err
		}
		_, err = rbClient.Create(ctx, &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:            PipelineRoleBinding,
				Namespace:       ns.Name,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			RoleRef:  wantRoleRef,
			Subjects: []rbacv1.Subject{wantSubject},
		}, metav1.CreateOptions{})
		return err
	}

	changed := false
	if !hasOwnerReference(existing.OwnerReferences, ownerRef) {
		existing.OwnerReferences = []metav1.OwnerReference{ownerRef}
		changed = true
	}
	if !hasSubject(existing.Subjects, wantSubject) {
		existing.Subjects = append(existing.Subjects, wantSubject)
		changed = true
	}
	if !changed {
		return nil
	}
	_, err = rbClient.Update(ctx, existing, metav1.UpdateOptions{})
	return err
}

// removeEditRoleBindingIfPresent deletes the openshift-pipelines-edit RoleBinding
// when createEditRoleBinding is disabled.
func (r *Reconciler) removeEditRoleBindingIfPresent(ctx context.Context, nsName string) error {
	err := r.kubeClient.RbacV1().RoleBindings(nsName).Delete(ctx, PipelineRoleBinding, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

// ---------------------------------------------------------------------------
// CA Bundles
// ---------------------------------------------------------------------------

// ensureCABundles creates the config-trusted-cabundle and config-service-cabundle
// ConfigMaps in the namespace (or strips owner references from existing ones).
// These ConfigMaps carry annotations/labels that trigger OpenShift's CA injection
// controllers so that pods can trust the cluster's CA and internal service CA.
func (r *Reconciler) ensureCABundles(ctx context.Context, ns *corev1.Namespace) error {
	logger := logging.FromContext(ctx)
	cmClient := r.kubeClient.CoreV1().ConfigMaps(ns.Name)

	// Ensure config-trusted-cabundle
	logger.Infof("Ensuring configmap %s in namespace %s", trustedCABundleConfigMap, ns.Name)
	trustedCM, err := cmClient.Get(ctx, trustedCABundleConfigMap, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if errors.IsNotFound(err) {
		trustedCM = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      trustedCABundleConfigMap,
				Namespace: ns.Name,
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "tekton-pipelines",
					// OpenShift CA injection label: cluster CA + user-provided certs
					"config.openshift.io/inject-trusted-cabundle": "true",
				},
			},
		}
		if _, err := cmClient.Create(ctx, trustedCM, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	} else {
		// Strip owner references from pre-existing ConfigMap to avoid GC.
		trustedCM.SetOwnerReferences(nil)
		if _, err := cmClient.Update(ctx, trustedCM, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	// Ensure config-service-cabundle
	logger.Infof("Ensuring configmap %s in namespace %s", serviceCABundleConfigMap, ns.Name)
	serviceCM, err := cmClient.Get(ctx, serviceCABundleConfigMap, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if errors.IsNotFound(err) {
		serviceCM = &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serviceCABundleConfigMap,
				Namespace: ns.Name,
				Labels: map[string]string{
					"app.kubernetes.io/part-of": "tekton-pipelines",
				},
				Annotations: map[string]string{
					// OpenShift service CA injection annotation: internal service certs
					"service.beta.openshift.io/inject-cabundle": "true",
				},
			},
		}
		if _, err := cmClient.Create(ctx, serviceCM, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	} else {
		serviceCM.SetOwnerReferences(nil)
		if _, err := cmClient.Update(ctx, serviceCM, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// Secret bindings
// ---------------------------------------------------------------------------

// ensureSecretBindings reconciles secret bindings on the pipeline SA:
//   - Adds secrets that match a binding rule and currently exist in the namespace.
//   - Removes secrets that were managed by a named binding but whose Secret was deleted.
//   - Removes secrets whose binding rule was changed or removed entirely from
//     spec.platforms.openshift.namespaceSync.secretBindings, by comparing against
//     the set of secret names this controller managed on the previous reconcile
//     (persisted in managedSecretBindingsAnnotation on the SA itself).
//   - Label-based bindings are cleaned up the same way: if a secret that used
//     to match a labelSelector stops matching it (deleted, or its label is
//     removed/changed) the reference is dropped on the next reconcile that
//     re-evaluates that binding, because the previous reconcile's match is
//     remembered via managedSecretBindingsAnnotation even though the binding
//     rule itself never changed. This relies on that namespace actually being
//     re-reconciled after the secret's labels change — see the Secret
//     informer's UpdateFunc in controller.go.
func (r *Reconciler) ensureSecretBindings(ctx context.Context, ns *corev1.Namespace, bindings []v1alpha1.SecretBinding) error {
	logger := logging.FromContext(ctx)

	sa, err := r.kubeClient.CoreV1().ServiceAccounts(ns.Name).Get(ctx, pipelineSA, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		// pipeline SA not yet created — SA watch will re-trigger this reconcile.
		return nil
	}
	if err != nil {
		return err
	}

	expected, currentlyManaged, err := r.resolveSecretBindingTargets(ctx, ns.Name, bindings)
	if err != nil {
		return err
	}

	// A name is stale — and must be dropped from the SA — if this controller
	// has managed it before (now, or on a previous reconcile per
	// managedSecretBindingsAnnotation) but it is no longer expected. This
	// covers a deleted/relabeled Secret, a changed binding rule, or the whole
	// secretBindings list being cleared from spec.
	stale := map[string]bool{}
	for name := range currentlyManaged {
		if !expected[name] {
			stale[name] = true
		}
	}
	for _, name := range parseManagedSecrets(sa.Annotations[managedSecretBindingsAnnotation]) {
		if !expected[name] {
			stale[name] = true
		}
	}

	pullKept, pullRemoved := removeStaleImagePullSecrets(sa.ImagePullSecrets, stale)
	secretKept, secretRemoved := removeStaleSecretRefs(sa.Secrets, stale)
	sa.ImagePullSecrets = pullKept
	sa.Secrets = secretKept

	changed := len(pullRemoved) > 0 || len(secretRemoved) > 0
	if changed {
		logger.Infof("Removing stale secret bindings %v from pipeline SA %s/%s", append(pullRemoved, secretRemoved...), ns.Name, pipelineSA)
	}

	// Add expected secrets that are not yet bound.
	for name := range expected {
		if bindSecretToSA(sa, name) {
			logger.Infof("Binding secret %s/%s to pipeline SA", ns.Name, name)
			changed = true
		}
	}

	newAnnotation := serializeManagedSecrets(currentlyManaged)
	if sa.Annotations[managedSecretBindingsAnnotation] != newAnnotation {
		changed = true
	}

	if !changed {
		return nil
	}

	imagePullSecrets := sa.ImagePullSecrets
	secretRefs := sa.Secrets

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		// Re-fetch on every attempt: reusing the ServiceAccount object from
		// outside this closure would retry with the same stale resourceVersion
		// and fail with the same conflict every time.
		latest, err := r.kubeClient.CoreV1().ServiceAccounts(ns.Name).Get(ctx, pipelineSA, metav1.GetOptions{})
		if err != nil {
			return err
		}
		latest.ImagePullSecrets = imagePullSecrets
		latest.Secrets = secretRefs
		if newAnnotation == "" {
			delete(latest.Annotations, managedSecretBindingsAnnotation)
		} else {
			if latest.Annotations == nil {
				latest.Annotations = map[string]string{}
			}
			latest.Annotations[managedSecretBindingsAnnotation] = newAnnotation
		}
		_, err = r.kubeClient.CoreV1().ServiceAccounts(ns.Name).Update(ctx, latest, metav1.UpdateOptions{})
		return err
	})
}

// resolveSecretBindingTargets evaluates bindings against live Secrets in the
// namespace and returns:
//   - expected: secret names that should be bound on the pipeline SA right now.
//   - currentlyManaged: every name this reconcile's rules reference, whether or
//     not the Secret currently exists. Persisted in managedSecretBindingsAnnotation
//     so a later reconcile can detect that a rule — or a labelSelector match —
//     went away, even though this reconcile's rules no longer mention that name.
func (r *Reconciler) resolveSecretBindingTargets(ctx context.Context, namespace string, bindings []v1alpha1.SecretBinding) (expected, currentlyManaged map[string]bool, err error) {
	expected = map[string]bool{}
	currentlyManaged = map[string]bool{}
	secretClient := r.kubeClient.CoreV1().Secrets(namespace)

	for _, binding := range bindings {
		switch {
		case binding.SecretName != "":
			currentlyManaged[binding.SecretName] = true
			_, getErr := secretClient.Get(ctx, binding.SecretName, metav1.GetOptions{})
			switch {
			case getErr == nil:
				expected[binding.SecretName] = true
			case !errors.IsNotFound(getErr):
				return nil, nil, getErr
			}

		case binding.LabelSelector != nil:
			sel, selErr := metav1.LabelSelectorAsSelector(binding.LabelSelector)
			if selErr != nil {
				return nil, nil, selErr
			}
			list, listErr := secretClient.List(ctx, metav1.ListOptions{LabelSelector: sel.String()})
			if listErr != nil {
				return nil, nil, listErr
			}
			for i := range list.Items {
				name := list.Items[i].Name
				currentlyManaged[name] = true
				expected[name] = true
			}
		}
	}
	return expected, currentlyManaged, nil
}

// removeStaleImagePullSecrets filters out refs whose name is in stale,
// returning the kept refs and the names that were dropped.
func removeStaleImagePullSecrets(refs []corev1.LocalObjectReference, stale map[string]bool) (kept []corev1.LocalObjectReference, removed []string) {
	kept = make([]corev1.LocalObjectReference, 0, len(refs))
	for _, ref := range refs {
		if stale[ref.Name] {
			removed = append(removed, ref.Name)
			continue
		}
		kept = append(kept, ref)
	}
	return kept, removed
}

// removeStaleSecretRefs is the corev1.ObjectReference equivalent of
// removeStaleImagePullSecrets, for sa.Secrets.
func removeStaleSecretRefs(refs []corev1.ObjectReference, stale map[string]bool) (kept []corev1.ObjectReference, removed []string) {
	kept = make([]corev1.ObjectReference, 0, len(refs))
	for _, ref := range refs {
		if stale[ref.Name] {
			removed = append(removed, ref.Name)
			continue
		}
		kept = append(kept, ref)
	}
	return kept, removed
}

// parseManagedSecrets splits the comma-separated managedSecretBindingsAnnotation
// value into secret names, skipping empty entries.
func parseManagedSecrets(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	names := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			names = append(names, p)
		}
	}
	return names
}

// serializeManagedSecrets renders the given set of secret names as a sorted,
// comma-separated string for storage in managedSecretBindingsAnnotation. Sorting
// keeps the annotation value stable across reconciles with the same managed set,
// avoiding no-op Update calls caused by map iteration order.
func serializeManagedSecrets(m map[string]bool) string {
	if len(m) == 0 {
		return ""
	}
	names := make([]string, 0, len(m))
	for name := range m {
		names = append(names, name)
	}
	sort.Strings(names)
	return strings.Join(names, ",")
}

// bindSecretToSA adds secretName to both imagePullSecrets and secrets on the SA
// if not already present. Returns true if the SA was modified.
func bindSecretToSA(sa *corev1.ServiceAccount, secretName string) bool {
	changed := false

	hasPull := false
	for _, ref := range sa.ImagePullSecrets {
		if ref.Name == secretName {
			hasPull = true
			break
		}
	}
	if !hasPull {
		sa.ImagePullSecrets = append(sa.ImagePullSecrets, corev1.LocalObjectReference{Name: secretName})
		changed = true
	}

	hasMount := false
	for _, ref := range sa.Secrets {
		if ref.Name == secretName {
			hasMount = true
			break
		}
	}
	if !hasMount {
		sa.Secrets = append(sa.Secrets, corev1.ObjectReference{Name: secretName})
		changed = true
	}

	return changed
}

// ---------------------------------------------------------------------------
// ClusterInterceptors ClusterRoleBinding
// ---------------------------------------------------------------------------

// ensureClusterInterceptorsSubject manages this namespace's pipeline SA in the
// openshift-pipelines-clusterinterceptors ClusterRoleBinding.
//
// EventListeners use ClusterInterceptors (cluster-scoped) to validate webhook
// payloads. Because ClusterInterceptors are cluster-scoped, only a
// ClusterRoleBinding can grant access — a namespace RoleBinding is not enough.
// This method patches exactly one subject (add or remove) per namespace
// reconcile, replacing the legacy batch-rebuild approach in rbac.go.
//
//   - SA exists  → subject is added (idempotent).
//   - SA absent  → subject is removed (handles SA deletion self-healing).
func (r *Reconciler) ensureClusterInterceptorsSubject(ctx context.Context, nsName string, tc *v1alpha1.TektonConfig) error {
	logger := logging.FromContext(ctx)
	rbacClient := r.kubeClient.RbacV1()
	ownerRef := tektonConfigOwnerRef(tc)

	// Ensure the static ClusterRole exists (content never changes).
	if _, err := rbacClient.ClusterRoles().Get(ctx, clusterInterceptorsClusterRole, metav1.GetOptions{}); errors.IsNotFound(err) {
		logger.Infof("Creating ClusterRole %s", clusterInterceptorsClusterRole)
		_, err = rbacClient.ClusterRoles().Create(ctx, &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name:            clusterInterceptorsClusterRole,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			Rules: []rbacv1.PolicyRule{{
				APIGroups: []string{"triggers.tekton.dev"},
				Resources: []string{"clusterinterceptors"},
				Verbs:     []string{"get", "list", "watch"},
			}},
		}, metav1.CreateOptions{})
		if err != nil && !errors.IsAlreadyExists(err) {
			return err
		}
	}

	// Determine whether the pipeline SA currently exists in this namespace.
	_, saErr := r.kubeClient.CoreV1().ServiceAccounts(nsName).Get(ctx, pipelineSA, metav1.GetOptions{})
	saExists := saErr == nil
	if saErr != nil && !errors.IsNotFound(saErr) {
		return saErr
	}

	subject := rbacv1.Subject{
		Kind:      rbacv1.ServiceAccountKind,
		Name:      pipelineSA,
		Namespace: nsName,
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crb, err := rbacClient.ClusterRoleBindings().Get(ctx, clusterInterceptorsClusterRole, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			if !saExists {
				return nil // nothing to create when SA is absent
			}
			logger.Infof("Creating ClusterRoleBinding %s", clusterInterceptorsClusterRole)
			_, err = rbacClient.ClusterRoleBindings().Create(ctx, &rbacv1.ClusterRoleBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:            clusterInterceptorsClusterRole,
					OwnerReferences: []metav1.OwnerReference{ownerRef},
				},
				RoleRef: rbacv1.RoleRef{
					APIGroup: rbacv1.GroupName,
					Kind:     "ClusterRole",
					Name:     clusterInterceptorsClusterRole,
				},
				Subjects: []rbacv1.Subject{subject},
			}, metav1.CreateOptions{})
			return err
		}
		if err != nil {
			return err
		}

		changed := false
		if saExists {
			if !hasSubject(crb.Subjects, subject) {
				logger.Infof("Adding %s/pipeline to ClusterInterceptors binding", nsName)
				crb.Subjects = append(crb.Subjects, subject)
				changed = true
			}
		} else {
			newSubjects := make([]rbacv1.Subject, 0, len(crb.Subjects))
			for _, s := range crb.Subjects {
				if s.Kind == rbacv1.ServiceAccountKind && s.Name == pipelineSA && s.Namespace == nsName {
					logger.Infof("Removing %s/pipeline from ClusterInterceptors binding", nsName)
					changed = true
					continue
				}
				newSubjects = append(newSubjects, s)
			}
			crb.Subjects = newSubjects
		}

		if !changed {
			return nil
		}
		_, err = rbacClient.ClusterRoleBindings().Update(ctx, crb, metav1.UpdateOptions{})
		return err
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func shouldIgnoreNamespace(ns *corev1.Namespace) bool {
	if nsIgnoreRegex.MatchString(ns.Name) {
		return true
	}
	return ns.DeletionTimestamp != nil
}

// namespaceMatchesSelector returns true when the namespace should be synced
// according to cfg.NamespaceSelector. When no selector is configured every
// non-ignored namespace matches (opt-in all by default). Setting the selector
// to an empty matchLabels ({}) matches nothing, effectively disabling sync for
// all namespaces without touching the individual feature flags — note that
// metav1.LabelSelectorAsSelector treats an empty-but-non-nil selector as
// "match everything", so that case is special-cased below to get the intended
// "match nothing" behaviour. A malformed selector fails closed (matches no
// namespace) so a typo cannot accidentally widen sync to the whole cluster.
func namespaceMatchesSelector(ns *corev1.Namespace, cfg *v1alpha1.NamespaceSyncConfig) bool {
	if cfg.NamespaceSelector == nil {
		return true
	}
	if len(cfg.NamespaceSelector.MatchLabels) == 0 && len(cfg.NamespaceSelector.MatchExpressions) == 0 {
		return false
	}
	sel, err := metav1.LabelSelectorAsSelector(cfg.NamespaceSelector)
	if err != nil {
		return false
	}
	return sel.Matches(labels.Set(ns.Labels))
}

func tektonConfigOwnerRef(tc *v1alpha1.TektonConfig) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: tc.GroupVersionKind().GroupVersion().String(),
		Kind:       tc.GroupVersionKind().Kind,
		Name:       tc.Name,
		UID:        tc.UID,
	}
}

func hasOwnerReference(refs []metav1.OwnerReference, target metav1.OwnerReference) bool {
	for _, r := range refs {
		if r.APIVersion == target.APIVersion && r.Kind == target.Kind && r.Name == target.Name {
			return true
		}
	}
	return false
}

func hasSubject(subjects []rbacv1.Subject, target rbacv1.Subject) bool {
	for _, s := range subjects {
		if s.Kind == target.Kind && s.Name == target.Name && s.Namespace == target.Namespace {
			return true
		}
	}
	return false
}

// allNamespacesFromLister returns all non-ignored namespace names from the lister.
func allNamespacesFromLister(lister corelisterv1.NamespaceLister) []string {
	nsList, err := lister.List(labels.Everything())
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(nsList))
	for _, ns := range nsList {
		if !shouldIgnoreNamespace(ns) {
			names = append(names, ns.Name)
		}
	}
	return names
}
