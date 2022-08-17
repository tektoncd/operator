package tektonhub

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// checkIfInstallerSetExist checks if installer set exists for a component and return true/false based on it
// and if installer set which already exist is of older version then it deletes and return false to create a new
// installer set
func (r *Reconciler) checkIfInstallerSetExist(ctx context.Context, oc clientset.Interface, relVersion string, installerSetType string) (bool, error) {

	labels := r.getLabels(installerSetType)
	labelSelector, err := common.LabelSelector(labels)
	if err != nil {
		return false, err
	}

	compInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, r.operatorClientSet, labelSelector)
	if err != nil {
		return false, err
	}

	if compInstallerSet != "" {
		// if already created then check which version it is
		ctIs, err := oc.OperatorV1alpha1().TektonInstallerSets().
			Get(ctx, compInstallerSet, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		version, ok := ctIs.Annotations[v1alpha1.ReleaseVersionKey]
		if ok && version == relVersion {
			// if installer set already exist and release version is same
			// then ignore and move on
			return true, nil
		}

		// release version doesn't exist or is different from expected
		// deleted existing InstallerSet and create a new one

		err = oc.OperatorV1alpha1().TektonInstallerSets().
			Delete(ctx, compInstallerSet, metav1.DeleteOptions{})
		if err != nil {
			return false, err
		}
	}

	return false, nil
}

func createInstallerSet(ctx context.Context, oc clientset.Interface, th *v1alpha1.TektonHub,
	manifest mf.Manifest, releaseVersion, component, installerSetPrefix, namespace string, labels map[string]string, specHash string) error {

	is := makeInstallerSet(th, manifest, installerSetPrefix, releaseVersion, namespace, labels, specHash)
	if is == nil {
		return fmt.Errorf("Unable to create installerset")
	}

	createdIs, err := oc.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, is, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	if len(th.Status.HubInstallerSet) == 0 {
		th.Status.HubInstallerSet = map[string]string{}
	}

	// Update the status of addon with created installerSet name
	th.Status.HubInstallerSet[component] = createdIs.Name
	th.Status.SetVersion(releaseVersion)
	return nil
}

func makeInstallerSet(th *v1alpha1.TektonHub, manifest mf.Manifest, prefix, releaseVersion, namespace string, labels map[string]string, specHash string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(th, th.GetGroupVersionKind())

	tektonHubCRSpecHash, err := hash.Compute(th.Spec)
	if err != nil {
		return nil
	}

	is := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", prefix),
			Labels:       labels,
			Annotations: map[string]string{
				v1alpha1.ReleaseVersionKey:  releaseVersion,
				v1alpha1.TargetNamespaceKey: namespace,
				v1alpha1.LastAppliedHashKey: tektonHubCRSpecHash,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}

	if specHash != "" {
		is.Annotations[v1alpha1.DbSecretHash] = specHash
	}

	return is
}
