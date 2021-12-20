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

package tektoninstallerset

import (
	"path"
	"testing"

	mf "github.com/manifestival/manifestival"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/ptr"
)

func TestInjectOwner_CRDs(t *testing.T) {

	testData := path.Join("testdata", "test-crd.yaml")
	sourceManifest, _ := mf.ManifestFrom(mf.Recursive(testData))

	owners := []v1.OwnerReference{{
		APIVersion:         "example.test",
		Kind:               "1",
		Name:               "installer-set",
		UID:                "abcd",
		Controller:         ptr.Bool(true),
		BlockOwnerDeletion: ptr.Bool(true),
	}}

	manifest, err := sourceManifest.Transform(injectOwnerForCRDsAndNamespace(owners))
	if err != nil {
		t.Fatal("unexpected error: ", err)
	}
	crdsOwner := manifest.Resources()[0].GetOwnerReferences()
	if len(crdsOwner) == 0 {
		t.Fatal("failed to add owner")
	}

	// Must not add owner for crds
	manifest, err = sourceManifest.Transform(injectOwner(owners))
	if err != nil {
		t.Fatal("unexpected error: ", err)
	}
	crdsOwner = manifest.Resources()[0].GetOwnerReferences()
	if len(crdsOwner) != 0 {
		t.Fatal("added owner which is not expected")
	}

}

func TestInjectOwner_NonCRDs(t *testing.T) {

	testData := path.Join("testdata", "test-non-crd.yaml")
	sourceManifest, _ := mf.ManifestFrom(mf.Recursive(testData))

	owners := []v1.OwnerReference{{
		APIVersion:         "example.test",
		Kind:               "1",
		Name:               "installer-set",
		UID:                "abcd",
		Controller:         ptr.Bool(true),
		BlockOwnerDeletion: ptr.Bool(true),
	}}

	// Must not add owner as resource is non-crd
	manifest, err := sourceManifest.Transform(injectOwnerForCRDsAndNamespace(owners))
	if err != nil {
		t.Fatal("unexpected error: ", err)
	}
	crdsOwner := manifest.Resources()[0].GetOwnerReferences()
	if len(crdsOwner) != 0 {
		t.Fatal("added owner which is not expected")
	}

	manifest, err = sourceManifest.Transform(injectOwner(owners))
	if err != nil {
		t.Fatal("unexpected error: ", err)
	}
	crdsOwner = manifest.Resources()[0].GetOwnerReferences()
	if len(crdsOwner) == 0 {
		t.Fatal("failed to add owner")
	}

}
