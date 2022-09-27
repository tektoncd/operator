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

package client

import (
	"context"
	"fmt"
	"strings"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/logging"
)

func (i *InstallerSetClient) Create(ctx context.Context, comp v1alpha1.TektonComponent, manifest *mf.Manifest, isType string) ([]v1alpha1.TektonInstallerSet, error) {
	logger := logging.FromContext(ctx).With("kind", i.resourceKind, "type", isType)

	switch isType {
	case InstallerTypeMain:
		sets, err := i.makeMainSets(ctx, comp, manifest)
		if err != nil {
			logger.Errorf("installer set creation failed for main type: %v", err)
			return sets, err
		}
		return sets, nil

	case InstallerTypePre, InstallerTypePost:
		kind := strings.ToLower(strings.TrimPrefix(i.resourceKind, "Tekton"))
		isName := fmt.Sprintf("%s-%s-", kind, isType)

		iS, err := i.makeInstallerSet(ctx, comp, manifest, isName, isType)
		if err != nil {
			return nil, err
		}

		iS, err = i.clientSet.Create(ctx, iS, metav1.CreateOptions{})
		if err != nil {
			return nil, err
		}
		return []v1alpha1.TektonInstallerSet{*iS}, nil

	case InstallerTypeCustom:
	// TODO

	default:
		return nil, fmt.Errorf("invalid installer set type")
	}

	return nil, nil
}

func (i *InstallerSetClient) makeMainSets(ctx context.Context, comp v1alpha1.TektonComponent, manifest *mf.Manifest) ([]v1alpha1.TektonInstallerSet, error) {
	staticManifest := manifest.Filter(mf.Not(mf.ByKind("Deployment")))
	deploymentManifest := manifest.Filter(mf.ByKind("Deployment"))

	kind := strings.ToLower(strings.TrimPrefix(i.resourceKind, "Tekton"))
	staticName := fmt.Sprintf("%s-%s-%s-", kind, InstallerTypeMain, InstallerSubTypeStatic)

	staticIS, err := i.makeInstallerSet(ctx, comp, &staticManifest, staticName, InstallerTypeMain)
	if err != nil {
		return nil, err
	}
	staticIS, err = i.clientSet.Create(ctx, staticIS, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	deployName := fmt.Sprintf("%s-%s-%s-", kind, InstallerTypeMain, InstallerSubTypeDeployment)
	deploymentIS, err := i.makeInstallerSet(ctx, comp, &deploymentManifest, deployName, InstallerTypeMain)
	if err != nil {
		return nil, err
	}

	deploymentIS, err = i.clientSet.Create(ctx, deploymentIS, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return []v1alpha1.TektonInstallerSet{*staticIS, *deploymentIS}, nil
}

func (i *InstallerSetClient) makeInstallerSet(ctx context.Context, comp v1alpha1.TektonComponent, manifest *mf.Manifest, isName, isType string) (*v1alpha1.TektonInstallerSet, error) {
	specHash, err := hash.Compute(comp.GetSpec())
	if err != nil {
		return nil, err
	}

	transformedMf, err := i.filterAndTransform(ctx, manifest, comp)
	if err != nil {
		return nil, err
	}

	ownerRef := *metav1.NewControllerRef(comp, v1alpha1.SchemeGroupVersion.WithKind(i.resourceKind))
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: isName,
			Labels: map[string]string{
				v1alpha1.CreatedByKey:      i.resourceKind,
				v1alpha1.ReleaseVersionKey: i.releaseVersion,
				v1alpha1.InstallerSetType:  isType,
			},
			Annotations: map[string]string{
				v1alpha1.TargetNamespaceKey: comp.GetSpec().GetTargetNamespace(),
				v1alpha1.LastAppliedHashKey: specHash,
			},
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: transformedMf.Resources(),
		},
	}, nil
}
