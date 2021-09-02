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
	"os"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	util "github.com/tektoncd/operator/pkg/reconciler/common/testing"
)

const (
	VERSION = "0.15.2"
)

func TestGetLatestRelease(t *testing.T) {
	koPath := "testdata/kodata"
	os.Setenv(KoEnvKey, koPath)
	defer os.Unsetenv(KoEnvKey)

	version := latestRelease(&v1alpha1.TektonTrigger{})
	util.AssertEqual(t, version, VERSION)
}

func TestListReleases(t *testing.T) {
	koPath := "testdata/kodata"
	os.Setenv(KoEnvKey, koPath)
	defer os.Unsetenv(KoEnvKey)
	expectedVersionList := []string{"0.15.2", "0.14.3", "0.13.2"}

	version, err := allReleases(&v1alpha1.TektonTrigger{})
	util.AssertEqual(t, err, nil)
	util.AssertDeepEqual(t, version, expectedVersionList)
}

func TestManifestPath(t *testing.T) {
	koPath := "testdata/kodata"
	os.Setenv(KoEnvKey, koPath)
	defer os.Unsetenv(KoEnvKey)
	expectedPath := "testdata/kodata/tekton-trigger/0.15.2"

	path := manifestPath(VERSION, &v1alpha1.TektonTrigger{})
	util.AssertEqual(t, path, expectedPath)

	path = installedManifestPath(VERSION, &v1alpha1.TektonTrigger{})
	util.AssertEqual(t, path, expectedPath)

	path = installedManifestPath(VERSION, &v1alpha1.TektonAddon{})
	util.AssertNotEqual(t, path, expectedPath)

}
