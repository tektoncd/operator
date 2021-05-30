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
	"regexp"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"knative.dev/pkg/logging"

	mf "github.com/manifestival/manifestival"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	pipelinesSCCClusterRole  = "pipelines-scc-clusterrole"
	pipelinesSCCRoleBinding  = "pipelines-scc-rolebinding"
	pipelinesSCC             = "pipelines-scc"
	pipelineSA               = "pipeline"
	serviceCABundleConfigMap = "config-service-cabundle"
	trustedCABundleConfigMap = "config-trusted-cabundle"
	namespaceIgnorePattern   = "^(openshift|kube)-"
)

type rbac struct {
	kubeClientSet     kubernetes.Interface
	operatorClientSet clientset.Interface
	manifest          mf.Manifest
	ownerRef          metav1.OwnerReference
}

func (r *rbac) createResources(ctx context.Context) error {

	logger := logging.FromContext(ctx)

	// Maintaining a separate cluster role for the scc declaration.
	// to assist us in managing this the scc association in a
	// granular way.
	if err := r.ensurePipelinesSCClusterRole(ctx); err != nil {
		return err
	}

	namespaces, err := r.kubeClientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	re := regexp.MustCompile(namespaceIgnorePattern)

	for _, n := range namespaces.Items {

		if ignore := re.MatchString(n.GetName()); ignore {
			logger.Info("IGNORES Namespace: ", n.GetName())
			continue
		}

		logger.Infow("Inject CA bundle configmap in ", "Namespace", n.GetName())
		if err := r.ensureCABundles(ctx, &n); err != nil {
			return err
		}

		logger.Infow("Ensures Default SA in ", "Namespace", n.GetName())
		sa, err := r.ensureSA(ctx, &n)
		if err != nil {
			return err
		}

		if err := r.ensurePipelinesSCCRoleBinding(ctx, sa); err != nil {
			return err
		}

		if err := r.ensureRoleBindings(ctx, sa); err != nil {
			return err
		}
	}

	return nil
}

func (r *rbac) ensureCABundles(ctx context.Context, ns *corev1.Namespace) error {
	logger := logging.FromContext(ctx)
	cfgInterface := r.kubeClientSet.CoreV1().ConfigMaps(ns.Name)

	// Ensure trusted CA bundle
	logger.Infof("finding configmap: %s/%s", ns.Name, trustedCABundleConfigMap)
	caBundleCM, err := cfgInterface.Get(ctx, trustedCABundleConfigMap, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if err != nil && errors.IsNotFound(err) {
		logger.Infof("creating configmap %s in %s namespace", trustedCABundleConfigMap, ns.Name)
		if err := createTrustedCABundleConfigMap(ctx, cfgInterface, trustedCABundleConfigMap, ns.Name, r.ownerRef); err != nil {
			return err
		}
	}
	// set owner reference if not set
	if err == nil && len(caBundleCM.GetOwnerReferences()) == 0 {
		caBundleCM.SetOwnerReferences([]metav1.OwnerReference{r.ownerRef})
		if _, err := cfgInterface.Update(ctx, caBundleCM, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	// Ensure service CA bundle
	logger.Infof("finding configmap: %s/%s", ns.Name, serviceCABundleConfigMap)
	serviceCABundleCM, err := cfgInterface.Get(ctx, serviceCABundleConfigMap, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	if err != nil && errors.IsNotFound(err) {
		logger.Infof("creating configmap %s in %s namespace", serviceCABundleConfigMap, ns.Name)
		if err := createServiceCABundleConfigMap(ctx, cfgInterface, serviceCABundleConfigMap, ns.Name, r.ownerRef); err != nil {
			return err
		}
	}
	// set owner reference if not set
	if err == nil && len(serviceCABundleCM.GetOwnerReferences()) == 0 {
		serviceCABundleCM.SetOwnerReferences([]metav1.OwnerReference{r.ownerRef})
		if _, err := cfgInterface.Update(ctx, serviceCABundleCM, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func createTrustedCABundleConfigMap(ctx context.Context, cfgInterface v1.ConfigMapInterface, name, ns string, ownerRef metav1.OwnerReference) error {
	c := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "tekton-pipelines",
				// user-provided and system CA certificates
				"config.openshift.io/inject-trusted-cabundle": "true",
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
	}

	_, err := cfgInterface.Create(ctx, c, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func createServiceCABundleConfigMap(ctx context.Context, cfgInterface v1.ConfigMapInterface, name, ns string, ownerRef metav1.OwnerReference) error {
	c := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
			Labels: map[string]string{
				"app.kubernetes.io/part-of": "tekton-pipelines",
			},
			Annotations: map[string]string{
				// service serving certificates (required to talk to the internal registry)
				"service.beta.openshift.io/inject-cabundle": "true",
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
	}

	_, err := cfgInterface.Create(ctx, c, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}
	return nil
}

func (r *rbac) ensureSA(ctx context.Context, ns *corev1.Namespace) (*corev1.ServiceAccount, error) {
	logger := logging.FromContext(ctx)
	logger.Infof("finding sa: %s/%s", ns.Name, "pipeline")
	saInterface := r.kubeClientSet.CoreV1().ServiceAccounts(ns.Name)

	sa, err := saInterface.Get(ctx, pipelineSA, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return nil, err
	}
	if err != nil && errors.IsNotFound(err) {
		logger.Info("creating sa", "sa", pipelineSA, "ns", ns.Name)
		return createSA(ctx, saInterface, ns.Name, r.ownerRef)
	}

	if len(sa.GetOwnerReferences()) == 0 {
		sa.SetOwnerReferences([]metav1.OwnerReference{r.ownerRef})
		return saInterface.Update(ctx, sa, metav1.UpdateOptions{})
	}

	return sa, nil
}

func createSA(ctx context.Context, saInterface v1.ServiceAccountInterface, ns string, ownerRef metav1.OwnerReference) (*corev1.ServiceAccount, error) {
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pipelineSA,
			Namespace:       ns,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
	}

	sa, err := saInterface.Create(ctx, sa, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}

	return sa, nil
}

func (r *rbac) ensurePipelinesSCClusterRole(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	logger.Info("finding cluster role:", pipelinesSCCClusterRole)

	clusterRole := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pipelinesSCCClusterRole,
			OwnerReferences: []metav1.OwnerReference{r.ownerRef},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"security.openshift.io",
				},
				ResourceNames: []string{
					pipelinesSCC,
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
	_, err := rbacClient.ClusterRoles().Get(ctx, pipelinesSCCClusterRole, metav1.GetOptions{})

	if err != nil {
		if errors.IsNotFound(err) {
			_, err = rbacClient.ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
		}
		return err
	}
	_, err = rbacClient.ClusterRoles().Update(ctx, clusterRole, metav1.UpdateOptions{})
	return err
}

func (r *rbac) ensurePipelinesSCCRoleBinding(ctx context.Context, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)
	rbacClient := r.kubeClientSet.RbacV1()

	logger.Info("finding role-binding", pipelinesSCCRoleBinding)
	pipelineRB, rbErr := rbacClient.RoleBindings(sa.Namespace).Get(ctx, pipelinesSCCRoleBinding, metav1.GetOptions{})
	if rbErr != nil && !errors.IsNotFound(rbErr) {
		logger.Error(rbErr, "rbac get error", pipelinesSCCRoleBinding)
		return rbErr
	}

	logger.Info("finding cluster role:", pipelinesSCCClusterRole)
	if _, err := rbacClient.ClusterRoles().Get(ctx, pipelinesSCCClusterRole, metav1.GetOptions{}); err != nil {
		logger.Error(err, "finding cluster role failed:", pipelinesSCCClusterRole)
		return err
	}

	if rbErr != nil && errors.IsNotFound(rbErr) {
		return r.createSCCRoleBinding(ctx, sa)
	}

	logger.Info("found rbac", "subjects", pipelineRB.Subjects)
	return r.updateRoleBinding(ctx, pipelineRB, sa)
}

func (r *rbac) createSCCRoleBinding(ctx context.Context, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)
	rbacClient := r.kubeClientSet.RbacV1()

	logger.Info("create new rolebinding:", pipelinesSCCRoleBinding)
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pipelinesSCCRoleBinding,
			Namespace:       sa.Namespace,
			OwnerReferences: []metav1.OwnerReference{r.ownerRef},
		},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: pipelinesSCCClusterRole},
		Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}},
	}

	_, err := rbacClient.RoleBindings(sa.Namespace).Create(ctx, rb, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "creation of rolebinding failed:", pipelinesSCCRoleBinding)
	}
	return err
}

func (r *rbac) updateRoleBinding(ctx context.Context, rb *rbacv1.RoleBinding, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)

	subject := rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}

	hasSubject := hasSubject(rb.Subjects, subject)
	if !hasSubject {
		rb.Subjects = append(rb.Subjects, subject)
	}

	ownerRef := rb.GetOwnerReferences()
	if len(ownerRef) == 0 {
		rb.SetOwnerReferences([]metav1.OwnerReference{r.ownerRef})
	}

	if hasSubject && (len(ownerRef) != 0) {
		logger.Info("rolebinding is up to date", "action", "none")
		return nil
	}

	logger.Info("update existing rolebinding edit")
	rbacClient := r.kubeClientSet.RbacV1()

	_, err := rbacClient.RoleBindings(sa.Namespace).Update(ctx, rb, metav1.UpdateOptions{})
	if err != nil {
		logger.Error(err, "failed to update edit rb")
		return err
	}
	logger.Error(err, "successfully updated edit rb")
	return nil
}

func hasSubject(subjects []rbacv1.Subject, x rbacv1.Subject) bool {
	for _, v := range subjects {
		if v.Name == x.Name && v.Kind == x.Kind && v.Namespace == x.Namespace {
			return true
		}
	}
	return false
}

func (r *rbac) ensureRoleBindings(ctx context.Context, sa *corev1.ServiceAccount) error {
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

func (r *rbac) createRoleBinding(ctx context.Context, sa *corev1.ServiceAccount) error {
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
		ObjectMeta: metav1.ObjectMeta{
			Name:            "edit",
			Namespace:       sa.Namespace,
			OwnerReferences: []metav1.OwnerReference{r.ownerRef},
		},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "edit"},
		Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}},
	}

	_, err = rbacClient.RoleBindings(sa.Namespace).Create(ctx, rb, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "creation of 'edit' rolebinding failed, in Namespace", sa.GetNamespace())
	}
	return err
}
