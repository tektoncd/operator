/*
Copyright 2020 The Tekton Authors

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

package common

import (
	"context"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/manifestival/manifestival/fake"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDeleteObsoleteResources(t *testing.T) {
	t.Setenv(KoEnvKey, "testdata/kodata")
	client := fake.New()
	manifest, err := mf.NewManifest("testdata/test_delete_obsolete_resources/dummy.release.notags.yaml", mf.UseClient(client))
	if err != nil {
		t.Error(err)
	}
	// Save the manifest resources
	if err := manifest.Apply(); err != nil {
		t.Error(err)
	}
	// Grab the ConfigMaps, ensure we have at least 1
	cms := manifest.Filter(mf.ByKind("ConfigMap")).Resources()
	if len(cms) == 0 {
		t.Error("Where'd all the ConfigMaps go?!")
	}
	// Verify they exist in the "database"
	for _, cm := range cms {
		if _, err := manifest.Client.Get(&cm); err != nil {
			t.Error(err)
		}
	}
	deleteObsoleteResources := DeleteObsoleteResources(context.TODO(), &v1alpha1.TektonTrigger{},
		func(context.Context, v1alpha1.TektonComponent) (*mf.Manifest, error) {
			return &manifest, nil
		})
	nocms := manifest.Filter(mf.Not(mf.ByKind("ConfigMap")))
	err = deleteObsoleteResources(context.TODO(), &nocms, nil)
	util.AssertNoError(t, err)
	// Now verify all the ConfigMaps are gone
	for _, cm := range cms {
		if _, err := manifest.Client.Get(&cm); !errors.IsNotFound(err) {
			t.Errorf("ConfigMap %s should've been deleted!", cm.GetName())
		}
	}
	// And verify everything else is still there
	for _, cm := range nocms.Resources() {
		if _, err := manifest.Client.Get(&cm); err != nil {
			t.Error(err)
		}
	}
	// Now verify CRD's don't get deleted
	v1crds, _ := manifest.Transform(func(u *unstructured.Unstructured) error {
		if u.GetKind() == "CustomResourceDefinition" {
			u.SetAPIVersion("apiextensions.k8s.io/v1")
		}
		return nil
	})
	err = deleteObsoleteResources(context.TODO(), &v1crds, nil)
	util.AssertNoError(t, err)
	// And verify the old ones are still there
	for _, cm := range manifest.Filter(mf.CRDs).Resources() {
		if _, err := manifest.Client.Get(&cm); err != nil {
			t.Error(err)
		}
	}
}
