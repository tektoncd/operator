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

package tektoninstallerset

import (
	"context"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	versionedClients "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Creates the installerset using name
func CreateInstallerSetWithName(ctx context.Context, ci ComponentInstaller, name string) (*v1alpha1.TektonInstallerSet, error) {
	newISM := newTisMetaWithName(name)

	return createInstallerSet(ctx, ci, newISM)
}

// Creates the installerset using generate name
func CreateInstallerSetWithGenerateName(ctx context.Context, ci ComponentInstaller, namePrefix string) (*v1alpha1.TektonInstallerSet, error) {
	newISM := newTisMetaWithGenerateName(namePrefix)
	return createInstallerSet(ctx, ci, newISM)
}

// Create the installerset
func createInstallerSet(ctx context.Context, ci ComponentInstaller, tis *tisMeta) (*v1alpha1.TektonInstallerSet, error) {
	client := getTektonInstallerSetClient()
	return createInstallerSetWithClient(ctx, client, ci, tis)
}

func createInstallerSetWithClient(ctx context.Context, client versionedClients.TektonInstallerSetInterface, ci ComponentInstaller, tis *tisMeta) (*v1alpha1.TektonInstallerSet, error) {
	tis.config(ctx, ci)

	manifest, err := ci.GetManifest(ctx)
	if err != nil {
		return nil, err
	}

	is := makeInstallerSet(manifest, tis)

	createdIs, err := client.Create(ctx, is, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdIs, nil
}

func makeInstallerSet(manifest *mf.Manifest, mt *tisMeta) *v1alpha1.TektonInstallerSet {
	return &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            mt.Name,
			GenerateName:    mt.GenerateName,
			Labels:          mt.Labels,
			Annotations:     mt.Annotations,
			OwnerReferences: mt.OwnerReferences,
		},
		Spec: v1alpha1.TektonInstallerSetSpec{
			Manifests: manifest.Resources(),
		},
	}
}
