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
	"os"
	"path/filepath"

	security "github.com/openshift/client-go/security/clientset/versioned"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/logging"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func createInstallerSet(ctx context.Context, oc versioned.Interface, tc *v1alpha1.TektonConfig, releaseVersion string) error {

	// add pipelines-scc
	pipelinescc := &mf.Manifest{}
	pipelinesSCCLocation := filepath.Join(os.Getenv(common.KoEnvKey), "tekton-pipeline", "00-prereconcile")
	if err := common.AppendManifest(pipelinescc, pipelinesSCCLocation); err != nil {
		return err
	}

	is := makeInstallerSet(tc, releaseVersion)
	is.Spec.Manifests = pipelinescc.Resources()

	createdIs, err := oc.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, is, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		return err
	}

	if len(tc.Status.TektonInstallerSet) == 0 {
		tc.Status.TektonInstallerSet = map[string]string{}
	}

	// Update the status of tektonConfig with created installerSet name
	tc.Status.TektonInstallerSet[rbacInstallerSetType] = createdIs.Name
	return nil
}

func makeInstallerSet(tc *v1alpha1.TektonConfig, releaseVersion string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(tc, tc.GetGroupVersionKind())
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: rbacInstallerSetNamePrefix,
			Labels: map[string]string{
				v1alpha1.CreatedByKey:      createdByValue,
				v1alpha1.InstallerSetType:  rbacInstallerSetType,
				v1alpha1.ReleaseVersionKey: releaseVersion,
			},
			Annotations: map[string]string{
				v1alpha1.ReleaseVersionKey:  releaseVersion,
				v1alpha1.TargetNamespaceKey: tc.Spec.TargetNamespace,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
	}
}

func deleteInstallerSet(ctx context.Context, oc versioned.Interface, tc *v1alpha1.TektonConfig, component string) error {
	labelSelector, err := common.LabelSelector(rbacInstallerSetSelector)
	if err != nil {
		return err
	}
	err = oc.OperatorV1alpha1().TektonInstallerSets().DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return err
	}
	// clear the name of installer set from TektonConfig status
	delete(tc.Status.TektonInstallerSet, component)

	return nil
}

// checkIfInstallerSetExist checks if installer set exists for a component and return true/false based on it
// and if installer set which already exist is of older version then it deletes and return false to create a new
// installer set
func checkIfInstallerSetExist(ctx context.Context, oc versioned.Interface, relVersion string,
	tc *v1alpha1.TektonConfig) (*v1alpha1.TektonInstallerSet, error) {

	labelSelector, err := common.LabelSelector(rbacInstallerSetSelector)
	if err != nil {
		return nil, err
	}
	existingInstallerSet, err := tektoninstallerset.CurrentInstallerSetName(ctx, oc, labelSelector)
	if err != nil {
		return nil, err
	}
	if existingInstallerSet == "" {
		return nil, nil
	}

	// if already created then check which version it is
	ctIs, err := oc.OperatorV1alpha1().TektonInstallerSets().
		Get(ctx, existingInstallerSet, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	if version, ok := ctIs.Annotations[v1alpha1.ReleaseVersionKey]; ok && version == relVersion {
		// if installer set already exist and release version is same
		// then ignore and move on
		return ctIs, nil
	}

	// release version doesn't exist or is different from expected
	// deleted existing InstallerSet and create a new one

	err = oc.OperatorV1alpha1().TektonInstallerSets().
		Delete(ctx, existingInstallerSet, metav1.DeleteOptions{})
	if err != nil {
		return nil, err
	}
	return nil, v1alpha1.RECONCILE_AGAIN_ERR
}

// Note: this should become a generic func going forward when we start adding
// more OpenShift types
func getSecurityClient(ctx context.Context) *security.Clientset {
	restConfig, err := rest.InClusterConfig()
	if err != nil {
		logging.FromContext(ctx).Panic(err)
	}
	securityClient, err := security.NewForConfig(restConfig)
	if err != nil {
		logging.FromContext(ctx).Panic(err)
	}
	return securityClient
}
