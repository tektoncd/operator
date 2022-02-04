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

package tektonpipeline

import (
	"context"
	stdError "errors"
	"fmt"
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const createdByValue = "TektonPipeline"

// checkIfInstallerSetExist checks if installer set exists for a component and return true/false based on it
// and if installer set which already exist is of older version/ or if target namespace is different then it
// deletes and return false to create a new installer set
func checkIfInstallerSetExist(ctx context.Context, oc clientset.Interface, relVersion string,
	tp *v1alpha1.TektonPipeline, ls string) (bool, error) {

	// Check if installer set is already created
	installerSets, err := oc.OperatorV1alpha1().TektonInstallerSets().
		List(ctx, metav1.ListOptions{
			LabelSelector: ls,
		})
	if err != nil {
		return false, err
	}
	if len(installerSets.Items) == 0 {
		// only scenario where this function returns
		// false, nil
		return false, nil
	}

	// if the List query returns more than 1 InstallerSets,
	// then delete all of them as we are expecting only one
	// If there are more than one then it means the state is
	// corrupted/unexpected. So to get back to desired state
	// delete all installerSets that match the labelSelector
	if len(installerSets.Items) > 1 {
		err = oc.OperatorV1alpha1().TektonInstallerSets().
			DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
				LabelSelector: ls,
			})

		if err != nil {
			return false, err
		}
		return false, v1alpha1.RECONCILE_AGAIN_ERR
	}

	// if the InstallerSet already exists then check then validate it:
	// Check release version and target namespace
	// If anyone of this is not as expected then delete existing
	// InstallerSet and return false

	version, vOk := installerSets.Items[0].Labels[v1alpha1.ReleaseVersionKey]
	namespace, nOk := installerSets.Items[0].Annotations[v1alpha1.TargetNamespaceKey]

	if vOk && nOk {
		if version == relVersion && namespace == tp.Spec.TargetNamespace {
			// only scenarion where this function returns
			// true
			return true, nil
		}
	}

	// release version/ target namespace information doesn't exist in InstallerSet
	// or is different from expected
	// deleted existing InstallerSet and return false

	err = oc.OperatorV1alpha1().TektonInstallerSets().
		Delete(ctx, installerSets.Items[0].GetName(), metav1.DeleteOptions{})
	if err != nil {
		return false, err
	}

	return false, v1alpha1.RECONCILE_AGAIN_ERR
}

func createInstallerSet(ctx context.Context, oc clientset.Interface, tp *v1alpha1.TektonPipeline,
	manifest mf.Manifest, releaseVersion, component string) error {

	is := makeInstallerSet(tp, manifest, component, releaseVersion)

	createdIs, err := oc.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, is, metav1.CreateOptions{})
	if err != nil {
		return err
	}

	if len(tp.Status.ExtentionInstallerSets) == 0 {
		tp.Status.ExtentionInstallerSets = map[string]string{}
	}

	// Update the status of pipeline with created installerSet name
	tp.Status.ExtentionInstallerSets[component] = createdIs.Name

	_, err = oc.OperatorV1alpha1().TektonPipelines().
		UpdateStatus(ctx, tp, metav1.UpdateOptions{})
	if err != nil {
		return err
	}

	return stdError.New("ensuring TektonPipeline status update")
}

func makeInstallerSet(tp *v1alpha1.TektonPipeline, manifest mf.Manifest, installerSetType, releaseVersion string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(tp, tp.GetGroupVersionKind())
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", strings.ToLower(installerSetType)),
			Labels: map[string]string{
				v1alpha1.CreatedByKey:      createdByValue,
				v1alpha1.ReleaseVersionKey: releaseVersion,
				v1alpha1.InstallerSetType:  installerSetType,
			},
			Annotations: map[string]string{
				v1alpha1.ReleaseVersionKey:  releaseVersion,
				v1alpha1.TargetNamespaceKey: tp.Spec.TargetNamespace,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}

func deleteInstallerSet(ctx context.Context, oc clientset.Interface, ta *v1alpha1.TektonPipeline, component, labelSelector string) error {

	// delete the installer set
	err := oc.OperatorV1alpha1().TektonInstallerSets().
		DeleteCollection(ctx, metav1.DeleteOptions{},
			metav1.ListOptions{
				LabelSelector: labelSelector,
			})
	if err != nil {
		return err
	}

	// clear the name of installer set from TektonPipeline status
	delete(ta.Status.ExtentionInstallerSets, component)
	_, err = oc.OperatorV1alpha1().TektonPipelines().UpdateStatus(ctx, ta, metav1.UpdateOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	return nil
}
