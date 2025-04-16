/*
Copyright 2025 The Tekton Authors

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

package tektonchain

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektoninstallerset/client"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Reconciler) createSecretInstallerSet(ctx context.Context, tc *v1alpha1.TektonChain) (*v1alpha1.TektonInstallerSet, error) {

	manifest := r.manifest
	// filter only secret for this installerset as this needs
	// to be restored over upgrade
	manifest = manifest.Filter(mf.ByKind("Secret"))
	transformer := filterAndTransform(r.extension)
	if _, err := transformer(ctx, &manifest, tc); err != nil {
		tc.Status.MarkNotReady("transformation failed: " + err.Error())
		return nil, err
	}

	// generate installer set
	tis := makeInstallerSet(tc, manifest, secretChainInstallerset, "", r.operatorVersion)

	// Add annoation to secret installer set in case the generate secret signing is set to true
	if tc.Spec.GenerateSigningSecret {
		tis.Annotations[secretTISSigningAnnotation] = "true"
	}

	// create installer set
	createdIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, tis, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdIs, nil
}

func (r *Reconciler) createConfigInstallerSet(ctx context.Context, tc *v1alpha1.TektonChain) (*v1alpha1.TektonInstallerSet, error) {
	manifest := r.manifest

	// remove secret from this installerset as this installerset will be deleted on upgrade
	manifest = manifest.Filter(mf.ByKind("ConfigMap"), mf.ByName("chains-config"))
	transformer := filterAndTransform(r.extension)
	if _, err := transformer(ctx, &manifest, tc); err != nil {
		tc.Status.MarkNotReady("transformation failed: " + err.Error())
		return nil, err
	}

	// generate installer set
	tis := makeInstallerSet(tc, manifest, configChainInstallerset, "", r.operatorVersion)

	// compute the hash of tektonchain spec and store as an annotation
	// in further reconciliation we compute hash of tc spec and check with
	// annotation, if they are same then we skip updating the object
	// otherwise we update the manifest
	specHash, err := hash.Compute(tc.Spec)
	if err != nil {
		return nil, err
	}
	tis.Annotations[v1alpha1.LastAppliedHashKey] = specHash

	// create installer set
	createdIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, tis, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdIs, nil

}

func (r *Reconciler) createInstallerSet(ctx context.Context, tc *v1alpha1.TektonChain) (*v1alpha1.TektonInstallerSet, error) {

	manifest := r.manifest
	// installerSet adds it's owner as namespace's owner
	// so deleting tekton chain deletes target namespace too
	// to skip it we filter out namespace if pipeline have same namespace
	pipelineNamespace, err := common.PipelineTargetNamspace(r.pipelineInformer)
	if err != nil {
		return nil, err
	}
	if tc.Spec.GetTargetNamespace() == pipelineNamespace {
		manifest = manifest.Filter(mf.Not(mf.ByKind("Namespace")))
	}

	// remove secret and `chains-config` configMap from this installerset as this installerset will be deleted on upgrade
	manifest = manifest.Filter(mf.Not(mf.ByKind("Secret")),
		mf.Not(mf.All(mf.ByName("chains-config"), mf.ByKind("ConfigMap"))))

	transformer := filterAndTransform(r.extension)
	if _, err = transformer(ctx, &manifest, tc); err != nil {
		tc.Status.MarkNotReady("transformation failed: " + err.Error())
		return nil, err
	}

	// set installerSet installType: deployment or statefulset
	installerSetInstallType := client.InstallerSubTypeDeployment
	if tc.Spec.Performance.StatefulsetOrdinals != nil && *tc.Spec.Performance.StatefulsetOrdinals {
		installerSetInstallType = client.InstallerSubTypeStatefulset
	}
	// generate installer set
	tis := makeInstallerSet(tc, manifest, v1alpha1.ChainResourceName, installerSetInstallType, r.operatorVersion)

	// compute the hash of tektonchain spec and store as an annotation
	// in further reconciliation we compute hash of tc spec and check with
	// annotation, if they are same then we skip updating the object
	// otherwise we update the manifest
	specHash, err := hash.Compute(tc.Spec)
	if err != nil {
		return nil, err
	}
	tis.Annotations[v1alpha1.LastAppliedHashKey] = specHash

	// create installer set
	createdIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, tis, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdIs, nil
}

func makeInstallerSet(tc *v1alpha1.TektonChain, manifest mf.Manifest, installerSetType, installerSetInstallType, releaseVersion string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(tc, tc.GetGroupVersionKind())
	labels := map[string]string{
		v1alpha1.CreatedByKey:      createdByValue,
		v1alpha1.ReleaseVersionKey: releaseVersion,
		v1alpha1.InstallerSetType:  installerSetType,
	}

	if installerSetInstallType != "" {
		labels[v1alpha1.InstallerSetInstallType] = installerSetInstallType
	}
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", installerSetType),
			Labels:       labels,
			Annotations: map[string]string{
				v1alpha1.TargetNamespaceKey: tc.Spec.TargetNamespace,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}
