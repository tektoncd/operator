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
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
)

const (
	VERSION = "0.15.2"
)

func TestGetLatestRelease(t *testing.T) {
	koPath := "testdata/kodata"
	t.Setenv(KoEnvKey, koPath)

	version := latestRelease(&v1alpha1.TektonTrigger{})
	util.AssertEqual(t, version, VERSION)
}

func TestListReleases(t *testing.T) {
	koPath := "testdata/kodata"
	t.Setenv(KoEnvKey, koPath)
	expectedVersionList := []string{"0.15.2", "0.14.3", "0.13.2"}

	version, err := allReleases(&v1alpha1.TektonTrigger{})
	util.AssertEqual(t, err, nil)
	util.AssertDeepEqual(t, version, expectedVersionList)
}

func TestAppendManifest(t *testing.T) {

	// Case 1
	var manifest mf.Manifest
	err := AppendManifest(&manifest, "testdata/kodata/tekton-addon")
	if err != nil {
		t.Fatal("failed to read yaml: ", err)
	}

	if len(manifest.Resources()) != 3 {
		t.Fatalf("failed to find expected number of resource: %d found, expected 3", len(manifest.Resources()))
	}

	// Case 2
	var newManifest mf.Manifest
	err = AppendManifest(&newManifest, "testdata/kodata/tekton-addon/0.0.1")
	if err != nil {
		t.Fatal("failed to read yaml: ", err)
	}

	if len(newManifest.Resources()) != 1 {
		t.Fatalf("failed to find expected number of resource: %d found, expected 3", len(newManifest.Resources()))
	}
}
