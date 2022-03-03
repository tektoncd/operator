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

package tektonhub

import (
	"path"
	"strings"
	"testing"

	mf "github.com/manifestival/manifestival"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestUpdateDbDeployment(t *testing.T) {
	testData := path.Join("testdata", "update-db-deployment.yaml")

	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	newManifest, err := manifest.Transform(UpdateDbDeployment())
	assert.NilError(t, err)

	d := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(newManifest.Resources()[0].Object, d)
	assert.NilError(t, err)

	env := d.Spec.Template.Spec.Containers[0].Env
	assert.Equal(t, env[0].Name, "POSTGRESQL_DATABASE")

	mountPath := d.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath
	assert.Equal(t, mountPath, "/var/lib/pgsql/data")

	cmd := d.Spec.Template.Spec.Containers[0].ReadinessProbe.Exec.Command
	assert.Equal(t, strings.Contains(cmd[2], "POSTGRESQL_USER"), true)
}
