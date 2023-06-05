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
		DBUser:                "postgres",
		DBPassword:            "postgres",
		DBHost:                "localhost",
		DBName:                "test",
		DBPort:                5432,
		ServerPort:            12345,
		PrometheusPort:        12347,
		DBSSLMode:             "enable",
		DBEnableAutoMigration: true,
		TLSHostnameOverride:   "localhostTest",
		AuthDisable:           true,
		AuthImpersonate:       true,
		LogLevel:              "warn",
		LogsAPI:               true,
		LogsPath:              "/logs/test",
		LogsType:              "s3",
		LogsBufferSize:        12321,
		S3BucketName:          "test",
		S3Endpoint:            "test",
		S3HostnameImmutable:   true,
		S3Region:              "west",
		S3AccessKeyID:         "secret",
		S3SecretAccessKey:     "secret",
		S3MultiPartSize:       123,
	}

	manifest, err = manifest.Transform(updateApiConfig(prop))
	assert.NilError(t, err)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assert.NilError(t, err)

	assert.Equal(t, cm.Data["config"], `DB_USER=postgres
DB_PASSWORD=postgres
DB_HOST=localhost
DB_PORT=5432
SERVER_PORT=12345
PROMETHEUS_PORT=12347
DB_NAME=test
DB_SSLMODE=enable
DB_ENABLE_AUTO_MIGRATION=true
TLS_HOSTNAME_OVERRIDE=localhostTest
TLS_PATH=/etc/tls
AUTH_DISABLE=true
AUTH_IMPERSONATE=true
LOG_LEVEL=warn
LOGS_API=true
LOGS_TYPE=s3
LOGS_BUFFER_SIZE=12321
LOGS_PATH=/logs/test
S3_BUCKET_NAME=test
S3_ENDPOINT=test
S3_HOSTNAME_IMMUTABLE=true
S3_REGION=west
S3_ACCESS_KEY_ID=secret
S3_SECRET_ACCESS_KEY=secret
S3_MULTI_PART_SIZE=123`)
}
