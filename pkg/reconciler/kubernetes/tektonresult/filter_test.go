/*
Copyright 2023 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" B]>SIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package tektonresult

import (
	"path"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
)

func Test_filterExternalDB(t *testing.T) {
	testData := path.Join("testdata", "db-statefulset.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)
	num := len(manifest.Resources())
	assert.Equal(t, num, 4)
	assert.Equal(t, manifest.Resources()[0].GetName(), statefulSetDB)
	filterExternalDB(&v1alpha1.TektonResult{
		Spec: v1alpha1.TektonResultSpec{
			Result: v1alpha1.Result{
				ResultsAPIProperties: v1alpha1.ResultsAPIProperties{
					IsExternalDB: true,
				},
			},
		},
	}, &manifest)
	num = len(manifest.Resources())
	assert.Equal(t, num, 1)
	assert.Equal(t, manifest.Resources()[0].GetName(), statefulSetDB+"-external")
}
