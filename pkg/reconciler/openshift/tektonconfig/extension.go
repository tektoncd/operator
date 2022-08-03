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

package tektonconfig

import (
	"context"
	"fmt"
	"os"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonconfig/extension"
	openshiftPipeline "github.com/tektoncd/operator/pkg/reconciler/openshift/tektonpipeline"
	openshiftTrigger "github.com/tektoncd/operator/pkg/reconciler/openshift/tektontrigger"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
)

const (
	versionKey = "VERSION"
)

func OpenShiftExtension(ctx context.Context) common.Extension {
	return openshiftExtension{
		operatorClientSet: operatorclient.Get(ctx),
		kubeClientSet:     kubeclient.Get(ctx),
	}
}

type openshiftExtension struct {
	operatorClientSet versioned.Interface
	kubeClientSet     kubernetes.Interface
}

func (oe openshiftExtension) Transformers(comp v1alpha1.TektonComponent) []mf.Transformer {
	return []mf.Transformer{}
}
func (oe openshiftExtension) PreReconcile(ctx context.Context, tc v1alpha1.TektonComponent) error {

	config := tc.(*v1alpha1.TektonConfig)
	r := rbac{
		kubeClientSet:     oe.kubeClientSet,
		operatorClientSet: oe.operatorClientSet,
		version:           os.Getenv(versionKey),
		tektonConfig:      config,
	}

	// set openshift specific defaults
	openshiftPipeline.SetDefault(&config.Spec.Pipeline)
	openshiftTrigger.SetDefault(&config.Spec.Trigger.TriggersProperties)
	r.setDefault()

	// below code helps to retain state of pre-existing SA at the time of upgrade
	if existingSAWithOwnerRef(r.tektonConfig) {
		if err := changeOwnerRefOfPreExistingSA(ctx, r.kubeClientSet, *config); err != nil {
			return err
		}
		tcLabels := config.GetLabels()
		tcLabels[serviceAccountCreationLabel] = "true"
		config.SetLabels(tcLabels)
		if _, err := oe.operatorClientSet.OperatorV1alpha1().TektonConfigs().Update(ctx, config, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}

	createRBACResource := true
	for _, v := range config.Spec.Params {
		// check for param name and if its matches to createRbacResource
		// then disable auto creation of RBAC resources by deleting installerSet
		if v.Name == rbacParamName && v.Value == "false" {
			createRBACResource = false
			if err := deleteInstallerSet(ctx, r.operatorClientSet, r.tektonConfig, componentNameRBAC); err != nil {
				return err
			}
			// remove openshift-pipelines.tekton.dev/namespace-reconcile-version label from namespaces while deleting RBAC resources.
			if err := r.cleanUp(ctx); err != nil {
				return err
			}
		}
	}

	// TODO: Remove this after v0.55.0 release, by following a depreciation notice
	// --------------------
	if err := r.cleanUpRBACNameChange(ctx); err != nil {
		return err
	}
	// --------------------

	if createRBACResource {
		return r.createResources(ctx)
	}
	return nil
}

func (oe openshiftExtension) PostReconcile(ctx context.Context, comp v1alpha1.TektonComponent) error {
	configInstance := comp.(*v1alpha1.TektonConfig)

	if configInstance.Spec.Profile == v1alpha1.ProfileAll {
		if _, err := extension.EnsureTektonAddonExists(ctx, oe.operatorClientSet.OperatorV1alpha1().TektonAddons(), configInstance); err != nil {
			configInstance.Status.MarkComponentNotReady(fmt.Sprintf("TektonAddon: %s", err.Error()))
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	}

	if configInstance.Spec.Profile == v1alpha1.ProfileLite || configInstance.Spec.Profile == v1alpha1.ProfileBasic {
		return extension.EnsureTektonAddonCRNotExists(ctx, oe.operatorClientSet.OperatorV1alpha1().TektonAddons())
	}

	return nil
}
func (oe openshiftExtension) Finalize(ctx context.Context, comp v1alpha1.TektonComponent) error {
	configInstance := comp.(*v1alpha1.TektonConfig)
	if configInstance.Spec.Profile == v1alpha1.ProfileAll {
		if err := extension.EnsureTektonAddonCRNotExists(ctx, oe.operatorClientSet.OperatorV1alpha1().TektonAddons()); err != nil {
			return err
		}
	}

	r := rbac{
		kubeClientSet: oe.kubeClientSet,
		version:       os.Getenv(versionKey),
	}
	return r.cleanUp(ctx)
}

// configOwnerRef returns owner reference pointing to passed instance
func configOwnerRef(tc v1alpha1.TektonInstallerSet) metav1.OwnerReference {
	return *metav1.NewControllerRef(&tc, tc.GetGroupVersionKind())
}

// tektonConfigOwnerRef returns owner reference of tektonConfig
func tektonConfigOwnerRef(tc v1alpha1.TektonConfig) metav1.OwnerReference {
	return *metav1.NewControllerRef(&tc, tc.GetGroupVersionKind())
}

func changeOwnerRefOfPreExistingSA(ctx context.Context, kc kubernetes.Interface, tc v1alpha1.TektonConfig) error {
	allSAs, err := kc.CoreV1().ServiceAccounts("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, sa := range allSAs.Items {
		if sa.Name == "pipeline" && !nsRegex.MatchString(sa.Namespace) {
			// set tektonconfig ownerRef
			tcOwnerRef := tektonConfigOwnerRef(tc)
			sa.SetOwnerReferences([]metav1.OwnerReference{tcOwnerRef})
			if _, err := kc.CoreV1().ServiceAccounts(sa.Namespace).Update(ctx, &sa, metav1.UpdateOptions{}); err != nil {
				return err
			}
		}
	}
	return nil
}

// existingSAWithOwnerRef checks if openshift-pipelines.tekton.dev/sa-created label is present on tektonconfig
// we add this label from pipelines 1.8, and do not add tektoninstaller set as owner of serviceaccount created
// if label not present it means SA was created earlier and we need to remove ownerRef before we do the update
// this helps us to keep pre-existing SA as it is.
func existingSAWithOwnerRef(tc *v1alpha1.TektonConfig) bool {
	tcLabels := tc.GetLabels()
	_, ok := tcLabels[serviceAccountCreationLabel]
	return !ok
}
