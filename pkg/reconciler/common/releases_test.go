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
	VERSION          = "0.15.2"
	TEKTON_PIPELINES = "testdata/kodata/tekton-pipeline/" + VERSION + "/release.notags.yaml"
)

func TestGetLatestRelease(t *testing.T) {
	koPath := "testdata/kodata"

	tests := []struct {
		name      string
		component v1alpha1.TektonComponent
		expected  string
	}{
		{
			name:      "tekton-pipeline",
			component: &v1alpha1.TektonPipeline{},
			expected:  VERSION,
		},
	}

	os.Setenv(KoEnvKey, koPath)
	defer os.Unsetenv(KoEnvKey)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			version := latestRelease(test.component)
			util.AssertEqual(t, version, test.expected)
		})
	}
}

func TestListReleases(t *testing.T) {
	koPath := "testdata/kodata"

	tests := []struct {
		name      string
		component v1alpha1.TektonComponent
		expected  []string
	}{
		{
			name:      "tekton-pipeline",
			component: &v1alpha1.TektonPipeline{},
			expected:  []string{"0.15.2"},
		},
	}

	os.Setenv(KoEnvKey, koPath)
	defer os.Unsetenv(KoEnvKey)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			version, err := allReleases(test.component)
			util.AssertEqual(t, err, nil)
			util.AssertDeepEqual(t, version, test.expected)
		})
	}
}
