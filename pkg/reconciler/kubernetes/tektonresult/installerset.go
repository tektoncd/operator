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

package tektonresult

import (
	"context"
	"fmt"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (r *Reconciler) createInstallerSet(ctx context.Context, tr *v1alpha1.TektonResult) (*v1alpha1.TektonInstallerSet, error) {

	if err := r.transform(ctx, &r.manifest, tr); err != nil {
		tr.Status.MarkNotReady("transformation failed: " + err.Error())
		return nil, err
	}

	// compute the hash of tektonresult spec and store as an annotation
	// in further reconciliation we compute hash of td spec and check with
	// annotation, if they are same then we skip updating the object
	// otherwise we update the manifest
	specHash, err := hash.Compute(tr.Spec)
	if err != nil {
		return nil, err
	}

	// create installer set
	tis := r.makeInstallerSet(tr, r.manifest, specHash)
	createdIs, err := r.operatorClientSet.OperatorV1alpha1().TektonInstallerSets().
		Create(ctx, tis, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return createdIs, nil
}

func (r *Reconciler) makeInstallerSet(tr *v1alpha1.TektonResult, manifest mf.Manifest, trSpecHash string) *v1alpha1.TektonInstallerSet {
	ownerRef := *metav1.NewControllerRef(tr, tr.GetGroupVersionKind())
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", v1alpha1.ResultResourceName),
			Labels: map[string]string{
				v1alpha1.CreatedByKey:      createdByValue,
				v1alpha1.InstallerSetType:  v1alpha1.ResultResourceName,
				v1alpha1.ReleaseVersionKey: r.operatorVersion,
			},
			Annotations: map[string]string{
				v1alpha1.TargetNamespaceKey: tr.Spec.TargetNamespace,
				v1alpha1.LastAppliedHashKey: trSpecHash,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}
