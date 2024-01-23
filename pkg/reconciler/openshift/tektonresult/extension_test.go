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
	"os"
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	rbac "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestGetRouteManifest(t *testing.T) {
	os.Setenv(common.KoEnvKey, "notExist")
	_, err := getRouteManifest()
	if err == nil {
		t.Error("expected error, received no error")
	}

	os.Setenv(common.KoEnvKey, "testdata")
	mf, err := getRouteManifest()
	assertNoEror(t, err)

	cr := &rbac.ClusterRole{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(mf.Resources()[0].Object, cr)
	assertNoEror(t, err)

}

func assertNoEror(t *testing.T, err error) {
	t.Helper()

	if err != nil {
		t.Errorf("assertion failed; expected no error %v", err)
	}
}
