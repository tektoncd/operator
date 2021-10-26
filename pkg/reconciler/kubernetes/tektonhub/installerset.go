package tektonhub

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// checkIfInstallerSetExist checks if installer set exists for a component and return true/false based on it
// and if installer set which already exist is of older version then it deletes and return false to create a new
// installer set
func checkIfInstallerSetExist(ctx context.Context, oc clientset.Interface, relVersion string,
	th *v1alpha1.TektonHub, component string) (bool, error) {

	// Check if installer set is already created
	compInstallerSet, ok := th.Status.HubInstallerSet[component]
	if !ok {
		return false, nil
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
	manifest mf.Manifest, releaseVersion, component, installerSetPrefix, namespace string) error {

	is := makeInstallerSet(th, manifest, installerSetPrefix, releaseVersion, namespace)

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

	_, err = oc.OperatorV1alpha1().TektonHubs().
		UpdateStatus(ctx, th, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return nil
}

func makeInstallerSet(th *v1alpha1.TektonHub, manifest mf.Manifest, prefix, releaseVersion, namespace string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(th, th.GetGroupVersionKind())
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", prefix),
			Labels: map[string]string{
				v1alpha1.CreatedByKey:     createdByValue,
				v1alpha1.InstallerSetType: v1alpha1.HubResourceName,
				v1alpha1.Component:        prefix,
			},
			Annotations: map[string]string{
				v1alpha1.ReleaseVersionKey:  releaseVersion,
				v1alpha1.TargetNamespaceKey: namespace,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}
