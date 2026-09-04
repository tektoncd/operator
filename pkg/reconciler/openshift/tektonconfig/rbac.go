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

package tektonconfig

import (
	"context"
	"fmt"
	"regexp"

	security "github.com/openshift/client-go/security/clientset/versioned"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"github.com/tektoncd/operator/pkg/common"
	reconcilerCommon "github.com/tektoncd/operator/pkg/reconciler/common"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
)

const (
	// pipelinesSCCClusterRole is the cluster-scoped ClusterRole that grants
	// `use` on the default SCC configured in TektonConfig.  Its content is
	// dynamic (depends on TektonConfig.SCC.Default) so it cannot be a static
	// manifest and must be managed here.
	pipelinesSCCClusterRole = "pipelines-scc-clusterrole"

	// serviceAccountCreationLabel is placed on TektonConfig to record that
	// pre-existing pipeline SAs have had their owner references migrated.
	serviceAccountCreationLabel = "openshift-pipelines.tekton.dev/sa-created"

	// namespaceVersionLabel was applied by the old rbac.go batch loop to track
	// which namespace version had been reconciled.  It is still read by cleanUp
	// so that the label is removed on upgrade.
	namespaceVersionLabel = "openshift-pipelines.tekton.dev/namespace-reconcile-version"

	createdByValue             = "RBAC"
	componentNameRBAC          = "rhosp-rbac"
	rbacInstallerSetType       = "rhosp-rbac"
	rbacInstallerSetNamePrefix = "rhosp-rbac-"

	// rbacInstallerSetNameOld is the pre-v0.55 installer-set name, kept for cleanup.
	rbacInstallerSetNameOld = "rbac-resources"
)

var (
	rbacInstallerSetSelector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     createdByValue,
			v1alpha1.InstallerSetType: componentNameRBAC,
		},
	}

	// nsRegex filters system/platform namespaces when cleanUp removes the
	// legacy namespaceVersionLabel.
	nsRegex = regexp.MustCompile(reconcilerCommon.NamespaceIgnorePattern)
)

type rbac struct {
	kubeClientSet     kubernetes.Interface
	operatorClientSet clientset.Interface
	securityClientSet security.Interface
	ownerRef          metav1.OwnerReference
	version           string
	tektonConfig      *v1alpha1.TektonConfig
}

// cleanUp removes the legacy namespaceVersionLabel from all namespaces that
// carry it.  Called when the rhosp-rbac InstallerSet is (re-)created so that
// the next reconcile cycle re-evaluates every namespace.
func (r *rbac) cleanUp(ctx context.Context) error {
	labelSelector := fmt.Sprintf("%s = %s", namespaceVersionLabel, r.version)
	namespaces, err := r.kubeClientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return fmt.Errorf("failed to retrieve namespaces with labelSelector %s: %v", labelSelector, err)
	}
	for _, n := range namespaces.Items {
		lbls := n.GetLabels()
		delete(lbls, namespaceVersionLabel)
		n.SetLabels(lbls)
		if _, err := r.kubeClientSet.CoreV1().Namespaces().Update(ctx, &n, metav1.UpdateOptions{}); err != nil {
			return fmt.Errorf("failed to update namespace %s: %v", n.Name, err)
		}
	}
	return nil
}

// EnsureRBACInstallerSet creates or returns the rhosp-rbac TektonInstallerSet.
func (r *rbac) EnsureRBACInstallerSet(ctx context.Context) (*v1alpha1.TektonInstallerSet, error) {
	if err := r.removeObsoleteRBACInstallerSet(ctx); err != nil {
		return nil, err
	}

	rbacISet, err := checkIfInstallerSetExist(ctx, r.operatorClientSet, r.version, r.tektonConfig)
	if err != nil {
		return nil, err
	}
	if rbacISet != nil {
		return rbacISet, nil
	}

	// InstallerSet missing — remove version labels so every namespace is
	// re-evaluated by the NamespaceSyncController, then create a fresh set.
	if err := r.cleanUp(ctx); err != nil {
		return nil, err
	}
	if err := createInstallerSet(ctx, r.operatorClientSet, r.tektonConfig, r.version); err != nil {
		return nil, err
	}
	return nil, v1alpha1.RECONCILE_AGAIN_ERR
}

// ensurePreRequisites validates cluster-scoped SCC prerequisites and keeps
// the pipelines-scc-clusterrole ClusterRole up to date.
func (r *rbac) ensurePreRequisites(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	rbacISet, err := r.EnsureRBACInstallerSet(ctx)
	if err != nil {
		return err
	}
	r.ownerRef = configOwnerRef(*rbacISet)

	defaultSCC := r.tektonConfig.Spec.Platforms.OpenShift.SCC.Default
	if defaultSCC == "" {
		return fmt.Errorf("tektonConfig.Spec.Platforms.OpenShift.SCC.Default cannot be empty")
	}
	logger.Debugf("default SCC set to: %s", defaultSCC)
	if err := common.VerifySCCExists(ctx, defaultSCC, r.securityClientSet); err != nil {
		return fmt.Errorf("failed to verify scc %s exists, %w", defaultSCC, err)
	}

	prioritizedSCCList, err := common.GetSCCRestrictiveList(ctx, r.securityClientSet)
	if err != nil {
		return err
	}

	maxAllowedSCC := r.tektonConfig.Spec.Platforms.OpenShift.SCC.MaxAllowed
	if maxAllowedSCC != "" {
		if err := common.VerifySCCExists(ctx, maxAllowedSCC, r.securityClientSet); err != nil {
			return fmt.Errorf("failed to verify scc %s exists, %w", maxAllowedSCC, err)
		}
		isPriority, err := common.SCCAMoreRestrictiveThanB(prioritizedSCCList, defaultSCC, maxAllowedSCC)
		if err != nil {
			return err
		}
		logger.Infof("Is maxAllowed SCC: %s less restrictive than default SCC: %s? %t", maxAllowedSCC, defaultSCC, isPriority)
		if !isPriority {
			return fmt.Errorf("maxAllowed SCC: %s must be less restrictive than the default SCC: %s", maxAllowedSCC, defaultSCC)
		}
		logger.Debugf("maxAllowed SCC set to: %s", maxAllowedSCC)
	} else {
		logger.Debug("No maxAllowed SCC set in TektonConfig")
	}

	return r.ensurePipelinesSCClusterRole(ctx)
}

// createResources is called by PreReconcile.
// Per-namespace work (pipeline SA, CA bundles, SCC RoleBinding, edit
// RoleBinding, secret bindings, ClusterInterceptors subjects) is fully owned
// by the NamespaceSyncController.  rbac.go only manages cluster-scoped
// prerequisites.
func (r *rbac) createResources(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	if err := r.ensurePreRequisites(ctx); err != nil {
		logger.Errorf("error validating cluster-scoped prerequisites: %v", err)
		return err
	}
	return nil
}

// ensurePipelinesSCClusterRole creates or updates the pipelines-scc-clusterrole
// ClusterRole whose content depends on the dynamic TektonConfig.SCC.Default.
func (r *rbac) ensurePipelinesSCClusterRole(ctx context.Context) error {
	logger := logging.FromContext(ctx)
	logger.Debug("finding cluster role:", pipelinesSCCClusterRole)

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pipelinesSCCClusterRole,
			OwnerReferences: []metav1.OwnerReference{r.ownerRef},
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups:     []string{"security.openshift.io"},
			ResourceNames: []string{r.tektonConfig.Spec.Platforms.OpenShift.SCC.Default},
			Resources:     []string{"securitycontextconstraints"},
			Verbs:         []string{"use"},
		}},
	}

	rbacClient := r.kubeClientSet.RbacV1()
	if _, err := rbacClient.ClusterRoles().Get(ctx, pipelinesSCCClusterRole, metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			_, err = rbacClient.ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
		}
		return err
	}
	_, err := rbacClient.ClusterRoles().Update(ctx, clusterRole, metav1.UpdateOptions{})
	return err
}

// removeObsoleteRBACInstallerSet deletes the pre-v0.55 installer-set name if present.
func (r *rbac) removeObsoleteRBACInstallerSet(ctx context.Context) error {
	isClient := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets()
	if err := isClient.Delete(ctx, rbacInstallerSetNameOld, metav1.DeleteOptions{}); err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}
