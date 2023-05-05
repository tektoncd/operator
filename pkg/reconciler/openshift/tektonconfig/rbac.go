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
	"math"
	"regexp"
	"sort"
	"time"

	securityv1 "github.com/openshift/api/security/v1"
	sccSort "github.com/openshift/apiserver-library-go/pkg/securitycontextconstraints/util/sort"
	security "github.com/openshift/client-go/security/clientset/versioned"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	nsV1 "k8s.io/client-go/informers/core/v1"
	rbacV1 "k8s.io/client-go/informers/rbac/v1"
	"k8s.io/client-go/kubernetes"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"knative.dev/pkg/logging"
)

const (
	pipelinesSCCRole        = "pipelines-scc-role"
	pipelinesSCCClusterRole = "pipelines-scc-clusterrole"
	pipelinesSCCRoleBinding = "pipelines-scc-rolebinding"
	pipelineSA              = "pipeline"
	PipelineRoleBinding     = "openshift-pipelines-edit"
	// namespaceSCCAnnotation is used to set SCC for a given namespace
	namespaceSCCAnnotation = "operator.tekton.dev/scc"

	// TODO: Remove this after v0.55.0 release, by following a depreciation notice
	// --------------------
	pipelineRoleBindingOld  = "edit"
	rbacInstallerSetNameOld = "rbac-resources"
	// --------------------
	serviceCABundleConfigMap    = "config-service-cabundle"
	trustedCABundleConfigMap    = "config-trusted-cabundle"
	clusterInterceptors         = "openshift-pipelines-clusterinterceptors"
	namespaceVersionLabel       = "openshift-pipelines.tekton.dev/namespace-reconcile-version"
	createdByValue              = "RBAC"
	componentNameRBAC           = "rhosp-rbac"
	rbacInstallerSetType        = "rhosp-rbac"
	rbacInstallerSetNamePrefix  = "rhosp-rbac-"
	rbacParamName               = "createRbacResource"
	serviceAccountCreationLabel = "openshift-pipelines.tekton.dev/sa-created"
)

var (
	rbacInstallerSetSelector = metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.CreatedByKey:     createdByValue,
			v1alpha1.InstallerSetType: componentNameRBAC,
		},
	}
)

// Namespace Regex to ignore the namespace for creating rbac resources.
var nsRegex = regexp.MustCompile(common.NamespaceIgnorePattern)

type rbac struct {
	kubeClientSet     kubernetes.Interface
	operatorClientSet clientset.Interface
	securityClientSet *security.Clientset
	rbacInformer      rbacV1.ClusterRoleBindingInformer
	nsInformer        nsV1.NamespaceInformer
	ownerRef          metav1.OwnerReference
	version           string
	tektonConfig      *v1alpha1.TektonConfig
}

func (r *rbac) cleanUp(ctx context.Context) error {

	// fetch the list of all namespaces which have label
	// `openshift-pipelines.tekton.dev/namespace-reconcile-version: <release-version>`
	namespaces, err := r.kubeClientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s = %s", namespaceVersionLabel, r.version),
	})
	if err != nil {
		return err
	}
	// loop on namespaces and remove label if exist
	for _, n := range namespaces.Items {
		labels := n.GetLabels()
		delete(labels, namespaceVersionLabel)
		n.SetLabels(labels)
		if _, err := r.kubeClientSet.CoreV1().Namespaces().Update(ctx, &n, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

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
	// A new installer needs to be created
	// either because of operator version upgrade or installerSet gone missing;
	// therefore all relevant namespaces need to be reconciled for RBAC resources.
	// Hence, remove the necessary labels to ensure that the namespaces will be 'not skipped'
	// RBAC reconcile logic
	err = r.cleanUp(ctx)
	if err != nil {
		return nil, err
	}

	err = createInstallerSet(ctx, r.operatorClientSet, r.tektonConfig, r.version)
	if err != nil {
		return nil, err
	}
	return nil, v1alpha1.RECONCILE_AGAIN_ERR
}

func (r *rbac) setDefault() {
	var (
		found = false
	)

	for i, v := range r.tektonConfig.Spec.Params {
		if v.Name == rbacParamName {
			found = true
			// If the value set is invalid then set key to default value as true.
			if v.Value != "false" && v.Value != "true" {
				r.tektonConfig.Spec.Params[i].Value = "true"
			}
			break
		}
	}
	if !found {
		r.tektonConfig.Spec.Params = append(r.tektonConfig.Spec.Params, v1alpha1.Param{
			Name:  rbacParamName,
			Value: "true",
		})
	}
}

func (r *rbac) verifySCCExists(ctx context.Context, sccName string) error {
	_, err := r.securityClientSet.SecurityV1().SecurityContextConstraints().Get(ctx, sccName, metav1.GetOptions{})
	return err
}

// ensurePreRequisites validates the resources before creation
func (r *rbac) ensurePreRequisites(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	rbacISet, err := r.EnsureRBACInstallerSet(ctx)
	if err != nil {
		return err
	}
	r.ownerRef = configOwnerRef(*rbacISet)

	// make sure default SCC is in place
	defaultSCC := r.tektonConfig.Spec.Platforms.OpenShift.SCC.Default
	if defaultSCC == "" {
		// Should not really happen due to defaulting, but okay...
		return fmt.Errorf("tektonConfig.Spec.Platforms.OpenShift.SCC.Default cannot be empty")
	}
	logger.Infof("default SCC set to: %s", defaultSCC)
	if err := r.verifySCCExists(ctx, defaultSCC); err != nil {
		return err
	}

	prioritizedSCCList, err := r.getPrioritizedSCCList(ctx)
	if err != nil {
		return err
	}

	// validate maxAllowed SCC
	maxAllowedSCC := r.tektonConfig.Spec.Platforms.OpenShift.SCC.MaxAllowed
	if maxAllowedSCC != "" {
		if err := r.verifySCCExists(ctx, maxAllowedSCC); err != nil {
			return err
		}

		isPriority, err := sccAEqualORPriorityOverB(prioritizedSCCList, maxAllowedSCC, defaultSCC)
		if err != nil {
			return err
		}
		logger.Infof("Does SCC: %s have >= priority than SCC: %s? %t", maxAllowedSCC, defaultSCC, isPriority)
		if !isPriority {
			return fmt.Errorf("maxAllowed SCC: %s must have a higher priority over default SCC: %s", maxAllowedSCC, defaultSCC)
		}
		logger.Infof("maxAllowed SCC set to: %s", maxAllowedSCC)
	} else {
		logger.Info("No maxAllowed SCC set in TektonConfig")
	}

	// Maintaining a separate cluster role for the scc declaration.
	// to assist us in managing this the scc association in a
	// granular way.
	// We need to make sure the pipelines-scc-clusterrole is up-to-date
	// irrespective of the fact that we get reconcilable namespaces or not.
	if err := r.ensurePipelinesSCClusterRole(ctx); err != nil {
		return err
	}

	return nil
}

func (r *rbac) getNamespacesToBeReconciled(ctx context.Context) ([]corev1.Namespace, error) {
	logger := logging.FromContext(ctx)

	// fetch the list of all namespaces which doesn't have label
	// `openshift-pipelines.tekton.dev/namespace-reconcile-version: <release-version>`
	allNamespaces, err := r.kubeClientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var namespaces []corev1.Namespace
	for _, ns := range allNamespaces.Items {
		// ignore namespaces with name passing regex `^(openshift|kube)-`
		if ignore := nsRegex.MatchString(ns.GetName()); ignore {
			continue
		}

		// ignore namespaces with DeletionTimestamp set
		if ns.GetObjectMeta().GetDeletionTimestamp() != nil {
			continue
		}

		// We want to monitor namespaces with the SCC annotation set
		if ns.Annotations[namespaceSCCAnnotation] != "" {
			namespaces = append(namespaces, ns)
			continue
		}

		// Then we want to accept namespaces that have not been reconciled yet
		if ns.Labels[namespaceVersionLabel] != r.version {
			namespaces = append(namespaces, ns)
			continue
		}

		// Now we're left with namespaces that have already been reconciled.
		// We must make sure that the default SCC is in force via the ClusterRole.
		sccRoleBinding, err := r.kubeClientSet.RbacV1().RoleBindings(ns.Name).Get(ctx, pipelinesSCCRoleBinding, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		if sccRoleBinding.RoleRef.Kind != "ClusterRole" {
			logger.Infof("RoleBinding %s in namespace: %s should have CluterRole with default SCC, will reconcile again...", pipelinesSCCRoleBinding, ns.Name)
			namespaces = append(namespaces, ns)
			continue
		}
	}

	return namespaces, nil
}

func (r *rbac) getSCCRoleInNamespace(ns *corev1.Namespace) *rbacv1.RoleRef {
	nsAnnotations := ns.GetAnnotations()
	nsSCC := nsAnnotations[namespaceSCCAnnotation]
	// If SCC is requested by namespace annotation, then we need a Role
	if nsSCC != "" {
		return &rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     pipelinesSCCRole,
		}
	}
	// If no SCC annotation is present in the namespace, we will use the
	// pipelines-scc-clusterrole
	return &rbacv1.RoleRef{
		APIGroup: rbacv1.GroupName,
		Kind:     "ClusterRole",
		Name:     pipelinesSCCClusterRole,
	}
}

func (r *rbac) handleSCCInNamespace(ctx context.Context, ns *corev1.Namespace) error {
	logger := logging.FromContext(ctx)

	nsName := ns.GetName()
	nsAnnotations := ns.GetAnnotations()
	nsSCC := nsAnnotations[namespaceSCCAnnotation]

	// No SCC is requested in the namespace
	if nsSCC == "" {
		// If we don't have a namespace annotation, then we don't need a
		// Role in this namespace as we will bind to the ClusterRole.
		// This happens in cases when the SCC annotation was removed from
		// the namespace.
		_, err := r.kubeClientSet.RbacV1().Roles(nsName).Get(ctx, pipelinesSCCRole, metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		// If `err == nil` AND role was found, it means that role exists
		if !errors.IsNotFound(err) {
			logger.Infof("Found leftover role: %s in namespace: %s, deleting...", pipelinesSCCRole, nsName)
			err := r.kubeClientSet.RbacV1().Roles(nsName).Delete(ctx, pipelinesSCCRole, metav1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
		}
		// Don't proceed further if no SCC requested by namespace
		return nil
	}

	// We're here, so the namespace has actually requested an SCC
	logger.Infof("Namespace: %s has requested SCC: %s", nsName, nsSCC)

	// Make sure that SCC exists on cluster
	if err := r.verifySCCExists(ctx, nsSCC); err != nil {
		logger.Error(err)

		// Create an event in the namespace if the SCC does not exist
		eventErr := r.createSCCFailureEventInNamespace(ctx, nsName, nsSCC)
		if eventErr != nil {
			logger.Errorf("Failed to create SCC not found event in namepsace: %s", nsName)
			return eventErr
		}
		return err
	}

	// Make sure SCC requested in the namespace has a lower or equal priority
	// than the SCC mentioned in maxAllowed
	maxAllowedSCC := r.tektonConfig.Spec.Platforms.OpenShift.SCC.MaxAllowed
	if maxAllowedSCC != "" {
		prioritizedSCCList, err := r.getPrioritizedSCCList(ctx)
		if err != nil {
			return err
		}
		isPriority, err := sccAEqualORPriorityOverB(prioritizedSCCList, maxAllowedSCC, nsSCC)
		if err != nil {
			return err
		}
		logger.Infof("Does SCC: %s have >= priority than SCC: %s? %t", maxAllowedSCC, nsSCC, isPriority)
		if !isPriority {
			return fmt.Errorf("namespace: %s has requested SCC: %s, but it has a higher priority than 'maxAllowed' SCC: %s", nsName, nsSCC, maxAllowedSCC)
		}
	}

	// Make sure a Role exists with the SCC attached in the namespace
	if err := r.ensureSCCRoleInNamespace(ctx, nsName, nsSCC); err != nil {
		return err
	}

	return nil
}

func (r *rbac) createResources(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	if err := r.ensurePreRequisites(ctx); err != nil {
		logger.Errorf("Error validating resources: %v", err)
		return err
	}

	namespacesToBeReconciled, err := r.getNamespacesToBeReconciled(ctx)
	if err != nil {
		logger.Error(err)
		return err
	}
	logger.Debugf("RBAC: found %d namespaces to be reconciled", len(namespacesToBeReconciled))

	// remove and update namespaces from Cluster Interceptors
	if err := r.removeAndUpdateNSFromCI(ctx); err != nil {
		logger.Error(err)
		return err
	}

	for _, ns := range namespacesToBeReconciled {
		logger.Infow("Inject CA bundle configmap in ", "Namespace", ns.GetName())
		if err := r.ensureCABundles(ctx, &ns); err != nil {
			return err
		}

		logger.Infow("Ensures Default SA in ", "Namespace", ns.GetName())
		sa, err := r.ensureSA(ctx, &ns)
		if err != nil {
			return err
		}

		// If "operator.tekton.dev/scc" exists in the namespace, then bind
		// that SCC to the SA
		err = r.handleSCCInNamespace(ctx, &ns)
		if err != nil {
			return err
		}

		// We use a namespace scoped Role when SCC annotation is present, and
		// a cluster scoped ClusterRole when the default SCC is used
		roleRef := r.getSCCRoleInNamespace(&ns)
		if err := r.ensurePipelinesSCCRoleBinding(ctx, sa, roleRef); err != nil {
			return err
		}

		if err := r.ensureRoleBindings(ctx, sa); err != nil {
			return err
		}

		if err := r.ensureClusterRoleBindings(ctx, sa); err != nil {
			return err
		}

		// Add `openshift-pipelines.tekton.dev/namespace-reconcile-version` label to namespace
		// so that rbac won't loop on it again
		nsLabels := ns.GetLabels()
		if len(nsLabels) == 0 {
			nsLabels = map[string]string{}
		}
		nsLabels[namespaceVersionLabel] = r.version
		ns.SetLabels(nsLabels)
		if _, err := r.kubeClientSet.CoreV1().Namespaces().Update(ctx, &ns, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (r *rbac) createSCCFailureEventInNamespace(ctx context.Context, namespace string, scc string) error {
	logger := logging.FromContext(ctx)

	failureEvent := corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    "pipelines-scc-failure-",
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{r.ownerRef},
		},
		EventTime:           metav1.NewMicroTime(time.Now()),
		Reason:              "RequestedSCCNotFound",
		Type:                "Warning",
		Action:              "SCCNotUpdated",
		Message:             fmt.Sprintf("SCC '%s' requested in annotation '%s' not found, SCC not updated in the namespace", scc, namespaceSCCAnnotation),
		ReportingController: "openshift-pipelines-operator",
		ReportingInstance:   r.ownerRef.Name,
		InvolvedObject: corev1.ObjectReference{
			Kind:       "Namespace",
			Name:       namespace,
			APIVersion: "v1",
			Namespace:  namespace,
		},
	}

	logger.Infof("Creating SCC failure event in namespace: %s", namespace)
	_, err := r.kubeClientSet.CoreV1().Events(namespace).Create(ctx, &failureEvent, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func sccAEqualORPriorityOverB(prioritizedSCCList []*securityv1.SecurityContextConstraints, sccA string, sccB string) (bool, error) {
	var sccAIndex, sccBIndex int
	var sccAFound, sccBFound bool
	for i, scc := range prioritizedSCCList {
		if scc.Name == sccA {
			sccAFound = true
			sccAIndex = i
		}
		if scc.Name == sccB {
			sccBFound = true
			sccBIndex = i
		}
		if sccAFound && sccBFound {
			break
		}
	}

	if !sccAFound || !sccBFound {
		return false, fmt.Errorf("SCCs not found while looking up priorities, found SCC %s: %t, found SCC %s: %t", sccA, sccAFound, sccB, sccBFound)
	}

	return sccAIndex <= sccBIndex, nil
}

func (r *rbac) getPrioritizedSCCList(ctx context.Context) ([]*securityv1.SecurityContextConstraints, error) {
	logger := logging.FromContext(ctx)
	sccList, err := r.securityClientSet.SecurityV1().SecurityContextConstraints().List(ctx, metav1.ListOptions{})
	if err != nil {
		logger.Error("Error listing SCCs")
		return nil, err
	}
	var sccPointerList []*securityv1.SecurityContextConstraints
	for i := range sccList.Items {
		sccPointerList = append(sccPointerList, &sccList.Items[i])
	}

	// This will sort the sccPointerList in order of priority
	sort.Sort(sccSort.ByPriority(sccPointerList))
	return sccPointerList, nil
}

func (r *rbac) ensureCABundles(ctx context.Context, ns *corev1.Namespace) error {
	logger := logging.FromContext(ctx)
	cfgInterface := r.kubeClientSet.CoreV1().ConfigMaps(ns.Name)

	// Ensure trusted CA bundle
	logger.Infof("finding configmap: %s/%s", ns.Name, trustedCABundleConfigMap)
	caBundleCM, getErr := cfgInterface.Get(ctx, trustedCABundleConfigMap, metav1.GetOptions{})
	if getErr != nil && !errors.IsNotFound(getErr) {
		return getErr
	}

	if getErr != nil && errors.IsNotFound(getErr) {
		logger.Infof("creating configmap %s in %s namespace", trustedCABundleConfigMap, ns.Name)
		var err error
		if caBundleCM, err = createTrustedCABundleConfigMap(ctx, cfgInterface, trustedCABundleConfigMap, ns.Name, r.ownerRef); err != nil {
			return err
		}
	}

	// If config map already exist then update the owner ref
	if getErr == nil {
		// set owner reference if not set or update owner reference if different owners are set
		caBundleCM.SetOwnerReferences(r.updateOwnerRefs(caBundleCM.GetOwnerReferences()))

		if _, err := cfgInterface.Update(ctx, caBundleCM, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	// Ensure service CA bundle
	logger.Infof("finding configmap: %s/%s", ns.Name, serviceCABundleConfigMap)
	serviceCABundleCM, getErr := cfgInterface.Get(ctx, serviceCABundleConfigMap, metav1.GetOptions{})
	if getErr != nil && !errors.IsNotFound(getErr) {
		return getErr
	}

	if getErr != nil && errors.IsNotFound(getErr) {
		logger.Infof("creating configmap %s in %s namespace", serviceCABundleConfigMap, ns.Name)
		var err error
		if serviceCABundleCM, err = createServiceCABundleConfigMap(ctx, cfgInterface, serviceCABundleConfigMap, ns.Name, r.ownerRef); err != nil {
			return err
		}
	}

	// If config map already exist then update the owner ref
	if getErr == nil {
		// set owner reference if not set or update owner reference if different owners are set
		serviceCABundleCM.SetOwnerReferences(r.updateOwnerRefs(serviceCABundleCM.GetOwnerReferences()))

		if _, err := cfgInterface.Update(ctx, serviceCABundleCM, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func createTrustedCABundleConfigMap(ctx context.Context, cfgInterface v1.ConfigMapInterface,
	name, ns string, ownerRef metav1.OwnerReference) (*corev1.ConfigMap, error) {
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

	cm, err := cfgInterface.Create(ctx, c, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}
	return cm, nil
}

func createServiceCABundleConfigMap(ctx context.Context, cfgInterface v1.ConfigMapInterface,
	name, ns string, ownerRef metav1.OwnerReference) (*corev1.ConfigMap, error) {
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

	cm, err := cfgInterface.Create(ctx, c, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}
	return cm, nil
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
		logger.Info("creating sa ", pipelineSA, " ns", ns.Name)
		return createSA(ctx, saInterface, ns.Name, *r.tektonConfig)
	}

	// set tektonConfig ownerRef
	tcOwnerRef := tektonConfigOwnerRef(*r.tektonConfig)
	sa.SetOwnerReferences([]metav1.OwnerReference{tcOwnerRef})

	return saInterface.Update(ctx, sa, metav1.UpdateOptions{})
}

func createSA(ctx context.Context, saInterface v1.ServiceAccountInterface, ns string, tc v1alpha1.TektonConfig) (*corev1.ServiceAccount, error) {
	tcOwnerRef := tektonConfigOwnerRef(tc)
	sa := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pipelineSA,
			Namespace:       ns,
			OwnerReferences: []metav1.OwnerReference{tcOwnerRef},
		},
	}

	sa, err := saInterface.Create(ctx, sa, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return nil, err
	}

	tektonConfigLabels := tc.GetLabels()
	tektonConfigLabels[serviceAccountCreationLabel] = "true"
	tc.SetLabels(tektonConfigLabels)
	return sa, nil
}

// ensureSCCRoleInNamespace ensures that the SCC role exists in the namespace
func (r *rbac) ensureSCCRoleInNamespace(ctx context.Context, namespace string, scc string) error {
	logger := logging.FromContext(ctx)

	logger.Infof("finding role: %s in namespace %s", pipelinesSCCRole, namespace)

	sccRole := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pipelinesSCCRole,
			Namespace:       namespace,
			OwnerReferences: []metav1.OwnerReference{r.ownerRef},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{
					"security.openshift.io",
				},
				ResourceNames: []string{
					scc,
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
	if _, err := rbacClient.Roles(namespace).Get(ctx, pipelinesSCCRole, metav1.GetOptions{}); err != nil {
		// If the role does not exist, then create it and exit
		if errors.IsNotFound(err) {
			_, err = rbacClient.Roles(namespace).Create(ctx, sccRole, metav1.CreateOptions{})
		}
		return err
	}
	// Update the role if it already exists
	_, err := rbacClient.Roles(namespace).Update(ctx, sccRole, metav1.UpdateOptions{})
	return err
}

// ensurePipelinesSCClusterRole ensures that `pipelines-scc` ClusterRole exists
// in the cluster. The SCC used in the ClusterRole is read from SCC config
// in TektonConfig (`pipelines-scc` by default)
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
					r.tektonConfig.Spec.Platforms.OpenShift.SCC.Default,
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
	if _, err := rbacClient.ClusterRoles().Get(ctx, pipelinesSCCClusterRole, metav1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			_, err = rbacClient.ClusterRoles().Create(ctx, clusterRole, metav1.CreateOptions{})
		}
		return err
	}
	_, err := rbacClient.ClusterRoles().Update(ctx, clusterRole, metav1.UpdateOptions{})
	return err
}

func (r *rbac) ensurePipelinesSCCRoleBinding(ctx context.Context, sa *corev1.ServiceAccount, roleRef *rbacv1.RoleRef) error {
	logger := logging.FromContext(ctx)
	rbacClient := r.kubeClientSet.RbacV1()

	roleKind := roleRef.Kind
	roleName := roleRef.Name
	if roleRef.Kind == "Role" {
		logger.Infof("finding %s: %s", roleKind, roleName)
		if _, err := rbacClient.Roles(sa.Namespace).Get(ctx, roleName, metav1.GetOptions{}); err != nil {
			logger.Error(err, "finding %s failed: %s", roleKind, roleName)
			return err
		}
	} else if roleKind == "ClusterRole" {
		logger.Infof("finding %s: %s", roleKind, roleName)
		if _, err := rbacClient.ClusterRoles().Get(ctx, roleName, metav1.GetOptions{}); err != nil {
			logger.Error(err, "finding %s failed: %s", roleKind, roleName)
			return err
		}
	} else {
		return fmt.Errorf("incorrect value set for roleKind - %s, needs to be Role or ClusterRole", roleKind)
	}

	logger.Info("finding role-binding", pipelinesSCCRoleBinding)
	pipelineRB, rbErr := rbacClient.RoleBindings(sa.Namespace).Get(ctx, pipelinesSCCRoleBinding, metav1.GetOptions{})
	if rbErr != nil && !errors.IsNotFound(rbErr) {
		logger.Error(rbErr, "rbac get error", pipelinesSCCRoleBinding)
		return rbErr
	}

	if rbErr != nil && errors.IsNotFound(rbErr) {
		return r.createSCCRoleBinding(ctx, sa, roleRef)
	}

	// We cannot update RoleRef in a RoleBinding, we need to delete and
	// recreate the binding in that case
	if pipelineRB.RoleRef.Kind != roleKind || pipelineRB.RoleRef.Name != roleName {
		logger.Infof("Need to update RoleRef in RoleBinding %s in namespace: %s, deleting and recreating...", pipelinesSCCRoleBinding, sa.Namespace)
		err := rbacClient.RoleBindings(sa.Namespace).Delete(ctx, pipelinesSCCRoleBinding, metav1.DeleteOptions{})
		if err != nil {
			return err
		}
		return r.createSCCRoleBinding(ctx, sa, roleRef)
	}

	logger.Info("found rbac", "subjects", pipelineRB.Subjects)
	return r.updateRoleBinding(ctx, pipelineRB, sa, roleRef)
}

func (r *rbac) createSCCRoleBinding(ctx context.Context, sa *corev1.ServiceAccount, roleRef *rbacv1.RoleRef) error {
	logger := logging.FromContext(ctx)
	rbacClient := r.kubeClientSet.RbacV1()

	logger.Info("create new rolebinding:", pipelinesSCCRoleBinding)
	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            pipelinesSCCRoleBinding,
			Namespace:       sa.Namespace,
			OwnerReferences: []metav1.OwnerReference{r.ownerRef},
		},
		RoleRef:  *roleRef,
		Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}},
	}

	_, err := rbacClient.RoleBindings(sa.Namespace).Create(ctx, rb, metav1.CreateOptions{})
	if err != nil {
		logger.Error(err, "creation of rolebinding failed:", pipelinesSCCRoleBinding)
	}
	return err
}

func (r *rbac) updateRoleBinding(ctx context.Context, rb *rbacv1.RoleBinding, sa *corev1.ServiceAccount, roleRef *rbacv1.RoleRef) error {
	logger := logging.FromContext(ctx)

	subject := rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}

	hasSubject := hasSubject(rb.Subjects, subject)
	if !hasSubject {
		rb.Subjects = append(rb.Subjects, subject)
	}

	rb.RoleRef = *roleRef

	rbacClient := r.kubeClientSet.RbacV1()
	hasOwnerRef := hasOwnerRefernce(rb.GetOwnerReferences(), r.ownerRef)

	ownerRef := r.updateOwnerRefs(rb.GetOwnerReferences())
	rb.SetOwnerReferences(ownerRef)

	// If owners are different then we need to set from r.ownerRef and update the roleBinding.
	if !hasOwnerRef {
		if _, err := rbacClient.RoleBindings(sa.Namespace).Update(ctx, rb, metav1.UpdateOptions{}); err != nil {
			logger.Error(err, "failed to update edit rb")
			return err
		}
	}

	if hasSubject && (len(ownerRef) != 0) {
		logger.Info("rolebinding is up to date ", "action ", "none")
		return nil
	}

	logger.Infof("update existing rolebinding %s/%s", rb.Namespace, rb.Name)

	_, err := rbacClient.RoleBindings(sa.Namespace).Update(ctx, rb, metav1.UpdateOptions{})
	if err != nil {
		logger.Errorf("%v: failed to update rolebinding %s/%s", err, rb.Namespace, rb.Name)
		return err
	}
	logger.Infof("successfully updated rolebinding %s/%s", rb.Namespace, rb.Name)
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

func hasOwnerRefernce(old []metav1.OwnerReference, new metav1.OwnerReference) bool {
	for _, v := range old {
		if v.APIVersion == new.APIVersion && v.Kind == new.Kind && v.Name == new.Name {
			return true
		}
	}
	return false
}

func (r *rbac) ensureRoleBindings(ctx context.Context, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)

	logger.Infof("finding role-binding: %s/%s", sa.Namespace, PipelineRoleBinding)
	rbacClient := r.kubeClientSet.RbacV1()

	editRB, err := rbacClient.RoleBindings(sa.Namespace).Get(ctx, PipelineRoleBinding, metav1.GetOptions{})

	if err == nil {
		logger.Infof("found rolebinding %s/%s", editRB.Namespace, editRB.Name)
		return r.updateRoleBinding(ctx, editRB, sa, &rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "edit",
		})
	}

	if errors.IsNotFound(err) {
		return r.createRoleBinding(ctx, sa)
	}

	return err
}

func (r *rbac) createRoleBinding(ctx context.Context, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)

	logger.Infof("create new rolebinding %s/%s", sa.Namespace, sa.Name)
	rbacClient := r.kubeClientSet.RbacV1()

	logger.Info("finding clusterrole edit")
	if _, err := rbacClient.ClusterRoles().Get(ctx, "edit", metav1.GetOptions{}); err != nil {
		logger.Error(err, "getting clusterRole 'edit' failed")
		return err
	}

	rb := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            PipelineRoleBinding,
			Namespace:       sa.Namespace,
			OwnerReferences: []metav1.OwnerReference{r.ownerRef},
		},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: "edit"},
		Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}},
	}

	if _, err := rbacClient.RoleBindings(sa.Namespace).Create(ctx, rb, metav1.CreateOptions{}); err != nil {
		logger.Errorf("%v: failed creation of rolebinding %s/%s", err, rb.Namespace, rb.Name)
		return err
	}
	return nil
}

func (r *rbac) ensureClusterRoleBindings(ctx context.Context, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)

	rbacClient := r.kubeClientSet.RbacV1()
	logger.Info("finding cluster-role ", clusterInterceptors)
	if _, err := rbacClient.ClusterRoles().Get(ctx, clusterInterceptors, metav1.GetOptions{}); errors.IsNotFound(err) {
		if e := r.createClusterRole(ctx); e != nil {
			return e
		}
	}

	logger.Info("finding cluster-role-binding ", clusterInterceptors)

	viewCRB, err := rbacClient.ClusterRoleBindings().Get(ctx, clusterInterceptors, metav1.GetOptions{})

	if err == nil {
		logger.Infof("found clusterrolebinding %s", viewCRB.Name)
		return r.updateClusterRoleBinding(ctx, viewCRB, sa)
	}

	if errors.IsNotFound(err) {
		return r.createClusterRoleBinding(ctx, sa)
	}

	return err
}

func (r *rbac) removeAndUpdateNSFromCI(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	rbacClient := r.kubeClientSet.RbacV1()
	rb, err := r.rbacInformer.Lister().Get(clusterInterceptors)
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "failed to get"+clusterInterceptors)
		return err
	}
	if rb == nil {
		return nil
	}

	req, err := labels.NewRequirement(namespaceVersionLabel, selection.Equals, []string{r.version})
	if err != nil {
		logger.Error(err, "failed to create requirement: ")
		return err
	}

	namespaces, err := r.nsInformer.Lister().List(labels.NewSelector().Add(*req))
	if err != nil && !errors.IsNotFound(err) {
		logger.Error(err, "failed to list namespace: ")
		return err
	}

	nsMap := map[string]string{}
	for i := range namespaces {
		nsMap[namespaces[i].Name] = namespaces[i].Name
	}

	var update bool
	for i := 0; i <= len(rb.Subjects)-1; i++ {
		if len(nsMap) != len(rb.Subjects) {
			if _, ok := nsMap[rb.Subjects[i].Namespace]; !ok {
				rb.Subjects = removeIndex(rb.Subjects, i)
				update = true
			}
		}
	}
	if update {
		if _, err := rbacClient.ClusterRoleBindings().Update(ctx, rb, metav1.UpdateOptions{}); err != nil {
			logger.Error(err, "failed to update "+clusterInterceptors+" crb")
			return err
		}
		logger.Infof("successfully removed namespace and updated %s ", clusterInterceptors)
	}
	return nil
}

func removeIndex(s []rbacv1.Subject, index int) []rbacv1.Subject {
	return append(s[:index], s[index+1:]...)
}

func (r *rbac) updateClusterRoleBinding(ctx context.Context, rb *rbacv1.ClusterRoleBinding, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)

	subject := rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}

	hasSubject := hasSubject(rb.Subjects, subject)
	if !hasSubject {
		rb.Subjects = append(rb.Subjects, subject)
	}

	rbacClient := r.kubeClientSet.RbacV1()
	hasOwnerRef := hasOwnerRefernce(rb.GetOwnerReferences(), r.ownerRef)

	ownerRef := r.updateOwnerRefs(rb.GetOwnerReferences())
	rb.SetOwnerReferences(ownerRef)

	// If owners are different then we need to set from r.ownerRef and update the clusterRolebinding.
	if !hasOwnerRef {
		if _, err := rbacClient.ClusterRoleBindings().Update(ctx, rb, metav1.UpdateOptions{}); err != nil {
			logger.Error(err, "failed to update "+clusterInterceptors+" crb")
			return err
		}
	}

	if hasSubject && (len(ownerRef) != 0) {
		logger.Info("clusterrolebinding is up to date", "action", "none")
		return nil
	}

	logger.Info("update existing clusterrolebinding ", clusterInterceptors)

	if _, err := rbacClient.ClusterRoleBindings().Update(ctx, rb, metav1.UpdateOptions{}); err != nil {
		logger.Error(err, "failed to update "+clusterInterceptors+" crb")
		return err
	}
	logger.Info("successfully updated ", clusterInterceptors)
	return nil
}

func (r *rbac) createClusterRoleBinding(ctx context.Context, sa *corev1.ServiceAccount) error {
	logger := logging.FromContext(ctx)

	logger.Info("create new clusterrolebinding ", clusterInterceptors)
	rbacClient := r.kubeClientSet.RbacV1()

	logger.Info("finding clusterrole ", clusterInterceptors)
	if _, err := rbacClient.ClusterRoles().Get(ctx, clusterInterceptors, metav1.GetOptions{}); err != nil {
		logger.Error(err, " getting clusterRole "+clusterInterceptors+" failed")
		return err
	}

	crb := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:            clusterInterceptors,
			OwnerReferences: []metav1.OwnerReference{r.ownerRef},
		},
		RoleRef:  rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: clusterInterceptors},
		Subjects: []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: sa.Name, Namespace: sa.Namespace}},
	}

	if _, err := rbacClient.ClusterRoleBindings().Create(ctx, crb, metav1.CreateOptions{}); err != nil {
		logger.Error(err, " creation of "+clusterInterceptors+" failed")
		return err
	}
	return nil
}

func (r *rbac) createClusterRole(ctx context.Context) error {
	logger := logging.FromContext(ctx)

	logger.Info("create new clusterrole ", clusterInterceptors)
	rbacClient := r.kubeClientSet.RbacV1()

	cr := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name:            clusterInterceptors,
			OwnerReferences: []metav1.OwnerReference{r.ownerRef},
		},
		Rules: []rbacv1.PolicyRule{{
			APIGroups: []string{"triggers.tekton.dev"},
			Resources: []string{"clusterinterceptors"},
			Verbs:     []string{"get", "list", "watch"},
		}},
	}

	if _, err := rbacClient.ClusterRoles().Create(ctx, cr, metav1.CreateOptions{}); err != nil {
		logger.Error(err, "creation of "+clusterInterceptors+" clusterrole failed")
		return err
	}
	return nil
}

func (r *rbac) updateOwnerRefs(ownerRef []metav1.OwnerReference) []metav1.OwnerReference {
	if len(ownerRef) == 0 {
		return []metav1.OwnerReference{r.ownerRef}
	}

	for i, ref := range ownerRef {
		if ref.APIVersion != r.ownerRef.APIVersion || ref.Kind != r.ownerRef.Kind || ref.Name != r.ownerRef.Name {
			// if owner reference are different remove the existing oand override with r.ownerRef
			return r.removeAndUpdate(ownerRef, i)
		}
	}

	return ownerRef
}

func (r *rbac) removeAndUpdate(slice []metav1.OwnerReference, s int) []metav1.OwnerReference {
	ownerRef := append(slice[:s], slice[s+1:]...)
	ownerRef = append(ownerRef, r.ownerRef)
	return ownerRef
}

// TODO: Remove this after v0.55.0 release, by following a depreciation notice
// --------------------
// cleanUpRBACNameChange will check remove ownerReference: RBAC installerset from
// 'edit' rolebindings from all relevant namespaces
// it will also remove 'pipeline' sa from subject list as
// the new 'openshift-pipelines-edit' rolebinding
func (r *rbac) cleanUpRBACNameChange(ctx context.Context) error {
	rbacClient := r.kubeClientSet.RbacV1()

	// fetch the list of all namespaces
	namespaces, err := r.kubeClientSet.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for _, ns := range namespaces.Items {
		nsName := ns.GetName()

		// filter namespaces:
		// ignore ns with name passing regex `^(openshift|kube)-`
		if ignore := nsRegex.MatchString(nsName); ignore {
			continue
		}

		// check if "edit" rolebinding exists in "ns" namespace
		editRB, err := rbacClient.RoleBindings(ns.GetName()).
			Get(ctx, pipelineRoleBindingOld, metav1.GetOptions{})
		if err != nil {
			// if "edit" rolebinding does not exists in "ns" namesapce, then do nothing
			if errors.IsNotFound(err) {
				continue
			}
			return err
		}

		// check if 'pipeline' serviceaccount is listed as a subject in 'edit' rolebinding
		depSub := rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: pipelineSA, Namespace: nsName}
		subIdx := math.MinInt16
		for i, s := range editRB.Subjects {
			if s.Name == depSub.Name && s.Kind == depSub.Kind && s.Namespace == depSub.Namespace {
				subIdx = i
				break
			}
		}

		// if 'pipeline' serviceaccount is listed as a subject in 'edit' rolebinding
		// remove 'pipeline' serviceaccount from subject list
		if subIdx >= 0 {
			editRB.Subjects = append(editRB.Subjects[:subIdx], editRB.Subjects[subIdx+1:]...)
		}

		// if 'pipeline' serviceaccount was the only item in the subject list of 'edit' rolebinding,
		// then we can delete 'edit' rolebinding as nobody else is using it
		if len(editRB.Subjects) == 0 {
			if err := rbacClient.RoleBindings(nsName).Delete(ctx, editRB.GetName(), metav1.DeleteOptions{}); err != nil {
				return err
			}
			continue
		}

		// remove TektonInstallerSet ownerReferece from "edit" rolebinding
		ownerRefs := editRB.GetOwnerReferences()
		ownerRefIdx := math.MinInt16
		for i, ownerRef := range ownerRefs {
			if ownerRef.Kind == "TektonInstallerSet" {
				ownerRefIdx = i
				break
			}
		}
		if ownerRefIdx >= 0 {
			ownerRefs := append(ownerRefs[:ownerRefIdx], ownerRefs[ownerRefIdx+1:]...)
			editRB.SetOwnerReferences(ownerRefs)

		}

		// if ownerReference or subject was updated, then update editRB resource on cluster
		if ownerRefIdx < 0 && subIdx < 0 {
			continue
		}
		if _, err := rbacClient.RoleBindings(nsName).Update(ctx, editRB, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

// --------------------

// TODO: Remove this after v0.55.0 release, by following a depreciation notice
// --------------------
func (r *rbac) removeObsoleteRBACInstallerSet(ctx context.Context) error {
	isClient := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets()
	err := isClient.Delete(ctx, rbacInstallerSetNameOld, metav1.DeleteOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// --------------------
