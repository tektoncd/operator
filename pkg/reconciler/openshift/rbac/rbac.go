package rbac

import (
	"context"
	"fmt"
	"regexp"

	v1 "k8s.io/client-go/kubernetes/typed/core/v1"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	nsreconciler "knative.dev/pkg/client/injection/kube/reconciler/core/v1/namespace"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// Reconciler implements controller.Reconciler for TektonPipeline resources.
type Reconciler struct {
	// kubeClientSet allows us to talk to the k8s for core APIs
	kubeClientSet kubernetes.Interface
	// operatorClientSet allows us to configure operator objects
	operatorClientSet clientset.Interface
	// manifest is empty, but with a valid client and logger. all
	// manifests are immutable, and any created during reconcile are
	// expected to be appended to this one, obviating the passing of
	// client & logger
	manifest mf.Manifest
	// Platform-specific behavior to affect the transform
	extension common.Extension
}

// Check that our Reconciler implements controller.Reconciler
var _ nsreconciler.Interface = (*Reconciler)(nil)

const (
	pipelineAnyuid = "pipeline-anyuid"
	pipelineSA     = "pipeline"
)

// FinalizeKind removes all resources after deletion of a TektonPipelines.
func (r *Reconciler) FinalizeKind(ctx context.Context, original *v1alpha1.TektonPipeline) pkgreconciler.Event {
	logger := logging.FromContext(ctx)

	// List all TektonPipelines to determine if cluster-scoped resources should be deleted.
	tps, err := r.operatorClientSet.OperatorV1alpha1().TektonPipelines().List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list all TektonPipelines: %w", err)
	}

	for _, ks := range tps.Items {
		if ks.GetDeletionTimestamp().IsZero() {
			// Not deleting all TektonPipelines. Nothing to do here.
			return nil
		}
	}

	if err := r.extension.Finalize(ctx, original); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}
	logger.Info("Deleting cluster-scoped resources")
	manifest, err := r.installed(ctx, original)
	if err != nil {
		logger.Error("Unable to fetch installed manifest; no cluster-scoped resources will be finalized", err)
		return nil
	}
	if err := common.Uninstall(ctx, manifest); err != nil {
		logger.Error("Failed to finalize platform resources", err)
	}
	return nil
}

// ReconcileKind compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) ReconcileKind(ctx context.Context, ns *corev1.Namespace) pkgreconciler.Event {
	logger := logging.FromContext(ctx)
	logger.Infow("Reconciling Namespace: Platform Openshift", "status", ns.GetName())

	ignorePattern := "^(openshift|kube)-"
	if ignore, _ := regexp.MatchString(ignorePattern, ns.GetName()); ignore {
		logger.Infow("Reconciling Namespace: IGNORE", "status", ns.GetName())
		return nil
	}
	logger.Infow("Reconciling Default SA in ", "Namespace", ns.GetName())

	sa, err := r.ensureSA(ctx, ns)
	if err != nil {
		return err
	}

	// Maintaining a separate cluster role for the scc declaration.
	// to assist us in managing this the scc association in a
	// granular way.
	if err := r.ensureSCClusterRole(ctx); err != nil {
		return err
	}

	if err := r.ensureSCCRoleBinding(ctx, sa); err != nil {
		return err
	}

	if err := r.ensureRoleBindings(ctx, sa); err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) ensureSA(ctx context.Context, ns *corev1.Namespace) (*corev1.ServiceAccount, error) {
	logger := logging.FromContext(ctx)
	logger.Info("finding sa", "pipeline-sa", "ns", ns.Name)
	saInterface := r.kubeClientSet.CoreV1().ServiceAccounts(ns.Name)
	sa, err := saInterface.Get(ctx, pipelineSA, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if err != nil && errors.IsNotFound(err) {
		logger.Info("creating sa", "sa", pipelineSA, "ns", ns.Name)
		return createSA(ctx, saInterface, ns.Name)
	}

	return sa, nil
}

func (r *Reconciler) ensureRoleBindings(ctx context.Context, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)

	logger.Info("finding role-binding edit")
	rbacClient := r.kubeClientSet.RbacV1()

	editRB, err := rbacClient.RoleBindings(sa.Namespace).Get(ctx, "edit", metav1.GetOptions{})

	if err == nil {
		logger.Infof("found rolebinding %s/%s", editRB.Namespace, editRB.Name)
		return r.updateRoleBinding(ctx, editRB, sa)
	}

	if errors.IsNotFound(err) {
		return r.createRoleBinding(ctx, sa)
	}

	return err
}

func (r *Reconciler) createRoleBinding(ctx context.Context, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)

	logger.Info("create new rolebinding edit, in Namespace", sa.GetNamespace())
	rbacClient := r.kubeClientSet.RbacV1()

	logger.Info("finding clusterrole edit")
	_, err := rbacClient.ClusterRoles().Get(ctx, "edit", metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "getting clusterRole 'edit' failed")
		return err
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: "edit", Namespace: sa.Namespace},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "edit"},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}},
	}

	_, err = rbacClient.RoleBindings(sa.Namespace).Create(ctx, rb, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "creation of 'edit' rolebinding failed, in Namespace", sa.GetNamespace())
	}
	return err
}

func (r *Reconciler) installed(ctx context.Context, instance v1alpha1.TektonComponent) (*mf.Manifest, error) {
	// Create new, empty manifest with valid client and logger
	installed := r.manifest.Append()
	// TODO: add ingress, etc
	//stages := common.Stages{common.AppendInstalled, r.transform}
	stages := common.Stages{common.AppendInstalled}
	err := stages.Execute(ctx, &installed, instance)
	return &installed, err
}

func (r *Reconciler) ensureSCClusterRole(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	logger.Info("finding cluster role pipeline-anyuid")

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineAnyuid},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"security.openshift.io",
				},
				ResourceNames: []string{
					"anyuid",
				},
				Resources: []string{
					"securitycontextconstraints",
				},
				Verbs: []string{
					"use",
				},
			},
		},
	}

	rbacClient := r.kubeClientSet.RbacV1()
	_, err := rbacClient.ClusterRoles().Get(ctx, pipelineAnyuid, metav1.GetOptions{})

	if err != nil {
		if errors.IsNotFound(err) {
			_, err = rbacClient.ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
		}
		return err
	}
	_, err = rbacClient.ClusterRoles().Update(ctx, clusterRole, metav1.UpdateOptions{})
	return err
}

func (r *Reconciler) ensureSCCRoleBinding(ctx context.Context, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)

	logger.Info("finding role-binding pipeline-anyuid")
	rbacClient := r.kubeClientSet.RbacV1()
	pipelineRB, rbErr := rbacClient.RoleBindings(sa.Namespace).Get(ctx, pipelineAnyuid, metav1.GetOptions{})
	if rbErr != nil && !errors.IsNotFound(rbErr) {
		logger.Error(rbErr, "rbac pipeline-anyuid get error")
		return rbErr
	}

	logger.Info("finding cluster role pipeline-anyuid")
	if _, err := rbacClient.ClusterRoles().Get(ctx, pipelineAnyuid, metav1.GetOptions{}); err != nil {
		logger.Error(err, "finding pipeline-anyuid cluster role failed")
		return err
	}

	if rbErr != nil && errors.IsNotFound(rbErr) {
		return r.createSCCRoleBinding(ctx, sa)
	}

	logger.Info("found rbac", "subjects", pipelineRB.Subjects)
	return r.updateRoleBinding(ctx, pipelineRB, sa)
}

func (r *Reconciler) createSCCRoleBinding(ctx context.Context, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)

	logger.Info("create new rolebinding pipeline-anyuid")
	rbacClient := r.kubeClientSet.RbacV1()
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: pipelineAnyuid, Namespace: sa.Namespace},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: pipelineAnyuid},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}},
	}

	_, err := rbacClient.RoleBindings(sa.Namespace).Create(ctx, rb, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "creation of pipeline-anyuid rb failed")
	}
	return err
}

func hasSubject(subjects []rbacv1.Subject, x rbacv1.Subject) bool {
	for _, v := range subjects {
		if v.Name == x.Name && v.Kind == x.Kind && v.Namespace == x.Namespace {
			return true
		}
	}
	return false
}

func (r *Reconciler) updateRoleBinding(ctx context.Context, rb *rbacv1.RoleBinding, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)

	subject := rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}

	if hasSubject(rb.Subjects, subject) {
		logger.Info("rolebinding is up to date", "action", "none")
		return nil
	}

	logger.Info("update existing rolebinding edit")
	rbacClient := r.kubeClientSet.RbacV1()
	rb.Subjects = append(rb.Subjects, subject)
	_, err := rbacClient.RoleBindings(sa.Namespace).Update(ctx, rb, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "updation of edit rb failed")
		return err
	}
	logger.Error(err, "successfully updated edit rb")
	return nil
}

func createSA(ctx context.Context, saInterface v1.ServiceAccountInterface, ns string) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pipelineSA,
			Namespace: ns,
		},
	}

	sa, err := saInterface.Create(ctx, sa, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}

	return sa, nil
}
