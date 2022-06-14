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
	"github.com/tektoncd/operator/pkg/reconciler/shared/hash"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Generates the installerset with name
func GenerateInstallerSetWithName(ctx context.Context, ci ComponentInstaller, name string) (*v1alpha1.TektonInstallerSet, error) {
	newISM := newTisMetaWithName(name)
	return generateInstallerSet(ctx, ci, newISM)
}

// Generates the installerset with prefix name
func GenerateInstallerSetWithPrefixName(ctx context.Context, ci ComponentInstaller, namePrefix string) (*v1alpha1.TektonInstallerSet, error) {
	newISM := newTisMetaWithGenerateName(namePrefix)
	return generateInstallerSet(ctx, ci, newISM)
}

// Generates the installerset without applying on the cluster
func generateInstallerSet(ctx context.Context, ci ComponentInstaller, tis *tisMeta) (*v1alpha1.TektonInstallerSet, error) {
	tis.config(ctx, ci)

	manifest, err := ci.GetManifest(ctx)
	if err != nil {
		return nil, err
	}

	is, err := makeInstallerSet(manifest, tis)
	if err != nil {
		return nil, err
	}

	return is, nil
}

// Creates the installerset on the cluster
func Create(ctx context.Context, is *v1alpha1.TektonInstallerSet) (*v1alpha1.TektonInstallerSet, error) {
	client := getTektonInstallerSetClient()
	return createWithClient(ctx, client, is)
}

func createWithClient(ctx context.Context, client versionedClients.TektonInstallerSetInterface, is *v1alpha1.TektonInstallerSet) (*v1alpha1.TektonInstallerSet, error) {
	createdIs, err := client.Create(ctx, is, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}

	return createdIs, nil
}

func makeInstallerSet(manifest *mf.Manifest, mt *tisMeta) (*v1alpha1.TektonInstallerSet, error) {

	is := &v1alpha1.TektonInstallerSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:            mt.Name,
			GenerateName:    mt.GenerateName,
			Labels:          mt.Labels,
			Annotations:     mt.Annotations,
			OwnerReferences: mt.OwnerReferences,
		},
		Spec: installerSpec(manifest),
	}

	specHash, err := getHash(is.Spec)
	if err != nil {
		return nil, err
	}
	is.Annotations[v1alpha1.LastAppliedHashKey] = specHash

	return is, nil
}

// Returns the spec of Installerset
func installerSpec(manifest *mf.Manifest) v1alpha1.TektonInstallerSetSpec {
	return v1alpha1.TektonInstallerSetSpec{
		Manifests: manifest.Resources(),
	}
}

// Computes the hash using spec of TektonInstallerSet
func getHash(spec v1alpha1.TektonInstallerSetSpec) (string, error) {
	specHash, err := hash.Compute(spec)
	if err != nil {
		return "", err
	}
	return specHash, nil
}
