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

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	createdByValue = "TektonPipeline"
)

// checkIfInstallerSetExist checks if installer set exists for a component and return true/false based on it
// and if installer set which already exist is of older version/ or if target namespace is different then it
// deletes and return false to create a new installer set
func checkIfInstallerSetExist(ctx context.Context, oc clientset.Interface, relVersion string,
	tp *v1alpha1.TektonPipeline, component string) (bool, error) {

	// Check if installer set is already created
	compInstallerSet, ok := tp.Status.ExtentionInstallerSets[component]
	if !ok {
		return false, nil
	}

	if compInstallerSet != "" {
		// if already created then check which version it is
		ctIs, err := oc.OperatorV1alpha1().TektonInstallerSets().
			Get(ctx, compInstallerSet, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}

		// Check release version and target namespace
		// If anyone of this is not as expected then delete existing
		// installer set and create a new one

		version, vOk := ctIs.Annotations[tektoninstallerset.ReleaseVersionKey]
		namespace, nOk := ctIs.Annotations[tektoninstallerset.TargetNamespaceKey]

		if vOk && nOk {
			if version == relVersion && namespace == tp.Spec.TargetNamespace {
				// if installer set already exist
				// release version and target namespace is as expected
				// then ignore and move on
				return true, nil
			}
		}

		// release version/ target namespace doesn't exist or is different from expected
		// deleted existing InstallerSet and create a new one

		err = oc.OperatorV1alpha1().TektonInstallerSets().
			Delete(ctx, compInstallerSet, metav1.DeleteOptions{})
		if err != nil {
			return false, err
		}
	}

	return false, nil
}

func createInstallerSet(ctx context.Context, oc clientset.Interface, tp *v1alpha1.TektonPipeline,
	manifest mf.Manifest, releaseVersion, component, installerSetPrefix string) error {

	is := makeInstallerSet(tp, manifest, installerSetPrefix, releaseVersion)

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

func makeInstallerSet(tp *v1alpha1.TektonPipeline, manifest mf.Manifest, prefix, releaseVersion string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(tp, tp.GetGroupVersionKind())
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", prefix),
			Labels: map[string]string{
				tektoninstallerset.CreatedByKey: createdByValue,
			},
			Annotations: map[string]string{
				tektoninstallerset.ReleaseVersionKey:  releaseVersion,
				tektoninstallerset.TargetNamespaceKey: tp.Spec.TargetNamespace,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}

func deleteInstallerSet(ctx context.Context, oc clientset.Interface, ta *v1alpha1.TektonPipeline, component string) error {

	compInstallerSet, ok := ta.Status.ExtentionInstallerSets[component]
	if !ok {
		return nil
	}

	if compInstallerSet != "" {
		// delete the installer set
		err := oc.OperatorV1alpha1().TektonInstallerSets().
			Delete(ctx, ta.Status.ExtentionInstallerSets[component], metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}

		// clear the name of installer set from TektonPipeline status
		delete(ta.Status.ExtentionInstallerSets, component)
		_, err = oc.OperatorV1alpha1().TektonPipelines().UpdateStatus(ctx, ta, metav1.UpdateOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	return nil
}
