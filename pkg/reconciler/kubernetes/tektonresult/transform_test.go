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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"k8s.io/apimachinery/pkg/runtime"
)

func Test_enablePVCLogging(t *testing.T) {
	testData := path.Join("testdata", "api-deployment.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	deployment := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)
	prop := v1alpha1.ResultsAPIProperties{
		LogsAPI:        true,
		LogsType:       "File",
		LogsPath:       "logs",
		LoggingPVCName: "tekton-logs",
	}

	manifest, err = manifest.Transform(enablePVCLogging(prop))
	assert.NilError(t, err)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)

	assert.Equal(t, deployment.Spec.Template.Spec.Volumes[2].Name, "tekton-logs")
	assert.Equal(t, deployment.Spec.Template.Spec.Containers[0].VolumeMounts[2].Name, "tekton-logs")
}

func Test_updateApiConfig(t *testing.T) {
	testData := path.Join("testdata", "api-config.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assert.NilError(t, err)
	prop := v1alpha1.ResultsAPIProperties{
		DBUser:              "postgres",
		DBPassword:          "postgres",
		DBHost:              "localhost",
		TLSHostnameOverride: "localhostTest",
		DBPort:              5432,
		LogsAPI:             true,
	}

	manifest, err = manifest.Transform(updateApiConfig(prop))
	assert.NilError(t, err)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assert.NilError(t, err)

	assert.Equal(t, cm.Data["config"], `DB_USER=
DB_PASSWORD=postgres
DB_HOST=localhost
DB_PORT=5432
DB_NAME=
DB_SSLMODE=disable
DB_ENABLE_AUTO_MIGRATION=false
GRPC_PORT=50051
REST_PORT=8080
PROMETHEUS_PORT=9090
TLS_HOSTNAME_OVERRIDE=localhostTest
TLS_PATH=/etc/tls
NO_AUTH=false
LOG_LEVEL=debug
LOGS_API=true
LOGS_TYPE=File
LOGS_BUFFER_SIZE=0
LOGS_PATH=/logs
S3_BUCKET_NAME=
S3_ENDPOINT=
S3_HOSTNAME_IMMUTABLE=false
S3_REGION=
S3_ACCESS_KEY_ID=
S3_SECRET_ACCESS_KEY=
S3_MULTI_PART_SIZE=0`)
}
