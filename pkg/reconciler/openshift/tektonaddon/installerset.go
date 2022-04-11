/*
Copyright 2022 The Tekton Authors

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

package tektonaddon

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// checkIfInstallerSetExist checks if installer set exists for a component and return true/false based on it
// and if installer set which already exist is of older version then it deletes and return false to create a new
// installer set
func checkIfInstallerSetExist(ctx context.Context, oc clientset.Interface, relVersion string,
	labelSelector string) (bool, error) {

	installerSets, err := oc.OperatorV1alpha1().TektonInstallerSets().
		List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
	if err != nil {
		return false, err
	}

	if len(installerSets.Items) == 0 {
		return false, nil
	}

	if len(installerSets.Items) == 1 {
		// if already created then check which version it is
		version, ok := installerSets.Items[0].Labels[v1alpha1.ReleaseVersionKey]
		if ok && version == relVersion {
			// if installer set already exist and release version is same
			// then ignore and move on
			return true, nil
		}
	}

	// release version doesn't exist or is different from expected
	// deleted existing InstallerSet and create a new one
	// or there is more than one installerset (unexpected)
	if err = oc.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		}); err != nil {
		return false, err
	}

	return false, v1alpha1.RECONCILE_AGAIN_ERR
}

func createInstallerSet(ctx context.Context, oc clientset.Interface, ta *v1alpha1.TektonAddon,
	manifest mf.Manifest, releaseVersion, component, installerSetPrefix string) error {

	specHash, err := hash.Compute(ta.Spec)
	if err != nil {
		return err
	}

	is := makeInstallerSet(ta, manifest, installerSetPrefix, releaseVersion, component, specHash)

	if _, err := oc.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, is, metav1.CreateOptions{}); err != nil {
		return err
	}

	return v1alpha1.RECONCILE_AGAIN_ERR
}

func makeInstallerSet(ta *v1alpha1.TektonAddon, manifest mf.Manifest, prefix, releaseVersion, component, specHash string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(ta, ta.GetGroupVersionKind())
	labels := map[string]string{
		v1alpha1.CreatedByKey:      CreatedByValue,
		v1alpha1.InstallerSetType:  component,
		v1alpha1.ReleaseVersionKey: releaseVersion,
	}
	namePrefix := fmt.Sprintf("%s-", prefix)
	// special label to make sure no two versioned clustertask installerset exist
	// for all patch releases
	if component == VersionedClusterTaskInstallerSet {
		labels[v1alpha1.ReleaseMinorVersionKey] = getPatchVersionTrimmed(releaseVersion)
		namePrefix = fmt.Sprintf("%s%s-", namePrefix, formattedVersionMajorMinor(releaseVersion))
	}
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: namePrefix,
			Labels:       labels,
			Annotations: map[string]string{
				v1alpha1.TargetNamespaceKey: ta.Spec.TargetNamespace,
				v1alpha1.LastAppliedHashKey: specHash,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}

func (r *Reconciler) checkComponentStatus(ctx context.Context, labelSelector string) error {

	// Check if installer set is already created
	installerSets, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector,
		})

	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	// To make sure there won't be duplicate installersets.
	if len(installerSets.Items) == 1 {
		ready := installerSets.Items[0].Status.GetCondition(apis.ConditionReady)
		if ready == nil || ready.Status == corev1.ConditionUnknown {
			return fmt.Errorf("InstallerSet %s: waiting for installation", installerSets.Items[0].Name)
		} else if ready.Status == corev1.ConditionFalse {
			return fmt.Errorf("InstallerSet %s: ", ready.Message)
		}
	}
	return nil
}

func (r *Reconciler) deleteInstallerSet(ctx context.Context, labelSelector string) error {

	err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labelSelector,
		})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}
