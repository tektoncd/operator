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
	"encoding/json"
	"errors"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/common/legacybuilds"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	// migrationAnnotation is set on the ShipwrightBuild CR once the legacy
	// Shipwright installation has been handed off. Shutdown is still rechecked.
	migrationAnnotation = "operator.tekton.dev/openshift-builds-migration"
	migrationDone       = "shipwright-completed"

	// legacyNamespace is the namespace the standalone OpenShift Builds Operator
	// deployed into — operator pod, Shipwright controller/webhook, and CSI driver
	// DaemonSet all run here.
	// Source: builds-operator/internal/common/constant.go OpenShiftBuildNamespaceName
	legacyNamespace = "openshift-builds"

	// legacyOperatorDeployment is the Deployment name of the standalone operator.
	// Source: builds-operator/config/manager/manager.yaml + kustomize namePrefix
	legacyOperatorDeployment = "openshift-builds-operator"
)

// openshiftBuildGVR is the GroupVersionResource for the legacy OpenShiftBuild CR.
var openshiftBuildGVR = schema.GroupVersionResource{
	Group:    "operator.openshift.io",
	Version:  "v1alpha1",
	Resource: "openshiftbuilds",
}

// prepareLegacyInstallation stops the legacy manager before InstallerSets apply
// shared resources. Recheck on every reconcile, including after completion: OLM
// upgrades can introduce a new CSV and restart the old manager.
func prepareLegacyInstallation(ctx context.Context, kubeClient kubernetes.Interface, dynClient dynamic.Interface) error {
	return legacybuilds.Prepare(ctx, kubeClient, dynClient)
}

// migrateFromLegacyInstallation finishes the Shipwright handoff after the new
// InstallerSets are ready. The legacy parent, finalizers, namespace and RBAC are
// preserved: CSI still depends on them and its migration is independent.
func migrateFromLegacyInstallation(ctx context.Context, kubeClient kubernetes.Interface, dynClient dynamic.Interface, operatorClient versioned.Interface, sb *v1alpha1.ShipwrightBuild) error {
	if err := prepareLegacyInstallation(ctx, kubeClient, dynClient); err != nil {
		return err
	}
	_, err := dynClient.Resource(openshiftBuildGVR).Get(ctx, "cluster", metav1.GetOptions{})
	if isNoCRDError(err) {
		// A fresh installation has nothing to migrate or mark completed.
		return nil
	}
	if err != nil {
		return err
	}
	if sb.Spec.TargetNamespace != legacyNamespace {
		// Do not leave two controllers reconciling the same Builds/BuildRuns.
		if err := stopLegacyDeployment(ctx, kubeClient, "shipwright-build-controller"); err != nil {
			return err
		}
		webhook, err := kubeClient.AppsV1().Deployments(legacyNamespace).Get(ctx, "shipwright-build-webhook", metav1.GetOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		if err == nil {
			if err := legacybuilds.WaitForWebhookReady(ctx, kubeClient, sb.Spec.TargetNamespace, "shipwright-build-webhook"); err != nil {
				return err
			}
			if err := legacybuilds.WaitForConversionRouting(ctx, dynClient, sb.Spec.TargetNamespace, "shp-build-webhook",
				"builds.shipwright.io", "buildruns.shipwright.io", "buildstrategies.shipwright.io", "clusterbuildstrategies.shipwright.io"); err != nil {
				return err
			}
			if err := stopLegacyDeployment(ctx, kubeClient, webhook.Name); err != nil {
				return err
			}
		}
	}
	if sb.GetAnnotations()[migrationAnnotation] == migrationDone {
		return nil
	}
	return stampMigrationAnnotation(ctx, operatorClient, sb)
}

// isNoCRDError returns true only when the error means the CRD for the resource
// is not registered on this cluster (i.e. the legacy operator was never installed).
// It uses concrete type assertions so that transient transport errors (timeouts,
// network failures) are never silently swallowed.
func isNoCRDError(err error) bool {
	if err == nil {
		return false
	}
	if apierrors.IsNotFound(err) {
		return true
	}
	// dynamic client returns *apimeta.NoResourceMatchError or *apimeta.NoKindMatchError
	// when the CRD does not exist in the cluster's API discovery.
	var noResource *apimeta.NoResourceMatchError
	var noKind *apimeta.NoKindMatchError
	return errors.As(err, &noResource) || errors.As(err, &noKind)
}

// stopLegacyOperatorDeployment scales the old operator Deployment to zero to
// prevent it from reverting resources the new installation applies.
func stopLegacyOperatorDeployment(ctx context.Context, client kubernetes.Interface) error {
	return legacybuilds.StopDeployment(ctx, client, legacyOperatorDeployment)
}

func stopLegacyDeployment(ctx context.Context, client kubernetes.Interface, name string) error {
	return legacybuilds.StopDeployment(ctx, client, name)
}

var legacyCSVGVR = schema.GroupVersionResource{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "clusterserviceversions"}

// stampMigrationAnnotation patches the ShipwrightBuild CR with the
// migration-done annotation. Subsequent reconciles still verify shutdown.
func stampMigrationAnnotation(ctx context.Context, operatorClient versioned.Interface, sb *v1alpha1.ShipwrightBuild) error {
	patch, err := json.Marshal(map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				migrationAnnotation: migrationDone,
			},
		},
	})
	if err != nil {
		return err
	}
	_, err = operatorClient.OperatorV1alpha1().ShipwrightBuilds().Patch(
		ctx, sb.Name, types.MergePatchType, patch, metav1.PatchOptions{},
	)
	return err
}
