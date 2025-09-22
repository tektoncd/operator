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

	mf "github.com/manifestival/manifestival"
	"k8s.io/apimachinery/pkg/runtime"

	"fmt"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"

	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func Test_enablePVCLogging(t *testing.T) {
	testData := path.Join("testdata", "api-deployment.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	deployment := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)
	logsAPI := true
	prop := v1alpha1.ResultsAPIProperties{
		LogsAPI:        &logsAPI,
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

	boolVal := true
	intVal := int64(12345)
	limit := uint(100)
	cm := &corev1.ConfigMap{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assert.NilError(t, err)
	bufferDuration := uint(20)
	spec := v1alpha1.TektonResultSpec{
		Result: v1alpha1.Result{

			LokiStackProperties: v1alpha1.LokiStackProperties{
				LokiStackName:      "foo",
				LokiStackNamespace: "bar",
			},
			ResultsAPIProperties: v1alpha1.ResultsAPIProperties{
				DBHost:                              "localhost",
				DBName:                              "test",
				ServerPort:                          &intVal,
				DBSSLMode:                           "enable",
				DBSSLRootCert:                       "/etc/tls/db/ca.crt",
				DBEnableAutoMigration:               &boolVal,
				TLSHostnameOverride:                 "localhostTest",
				AuthDisable:                         &boolVal,
				AuthImpersonate:                     &boolVal,
				PrometheusPort:                      &intVal,
				PrometheusHistogram:                 &boolVal,
				LogLevel:                            "warn",
				LogsAPI:                             &boolVal,
				LogsPath:                            "/logs/test",
				LogsType:                            "s3",
				LogsBufferSize:                      &intVal,
				StorageEmulatorHost:                 "http://localhost:9004",
				LoggingPluginForwarderDelayDuration: &bufferDuration,
				LoggingPluginQueryLimit:             &limit,
				LoggingPluginQueryParams:            "direction=asc&skip=0",
				LoggingPluginMultipartRegex:         `-%s`,
			},
		},
	}

	manifest, err = manifest.Transform(updateApiConfig(spec))
	assert.NilError(t, err)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, cm)
	assert.NilError(t, err)

	assert.Equal(t, cm.Data["config"], `DB_HOST=localhost
DB_PORT=5432
SERVER_PORT=12345
PROMETHEUS_PORT=12345
PROMETHEUS_HISTOGRAM=true
DB_NAME=test
DB_SSLMODE=enable
DB_SSLROOTCERT=/etc/tls/db/ca.crt
DB_ENABLE_AUTO_MIGRATION=true
TLS_HOSTNAME_OVERRIDE=localhostTest
TLS_PATH=/etc/tls
AUTH_DISABLE=true
AUTH_IMPERSONATE=true
LOG_LEVEL=warn
LOGGING_PLUGIN_API_URL=https://foo-gateway-http.bar.svc.cluster.local:8080
LOGGING_PLUGIN_FORWARDER_DELAY_DURATION=20
LOGGING_PLUGIN_NAMESPACE_KEY=kubernetes_namespace_name
LOGGING_PLUGIN_PROXY_PATH=/api/logs/v1/application
LOGGING_PLUGIN_STATIC_LABELS=log_type=application
LOGGING_PLUGIN_TLS_VERIFICATION_DISABLE=false
LOGGING_PLUGIN_TOKEN_PATH=/var/run/secrets/kubernetes.io/serviceaccount/token
LOGS_API=true
LOGS_TYPE=s3
LOGS_BUFFER_SIZE=12345
LOGS_PATH=/logs/test
LOGGING_PLUGIN_QUERY_LIMIT=100
LOGGING_PLUGIN_QUERY_PARAMS=direction=asc&skip=0
LOGGING_PLUGIN_MULTIPART_REGEX=-%s
STORAGE_EMULATOR_HOST=http://localhost:9004`)
}

func Test_GoogleCred(t *testing.T) {
	testData := []string{path.Join("testdata", "api-deployment-gcs.yaml"), path.Join("testdata", "api-deployment.yaml")}
	logsAPI := true
	for i := range testData {
		manifest, err := mf.ManifestFrom(mf.Recursive(testData[i]))
		assert.NilError(t, err)

		deployment := &appsv1.Deployment{}
		err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
		assert.NilError(t, err)
		prop := v1alpha1.ResultsAPIProperties{
			LogsAPI:            &logsAPI,
			LogsType:           "GCS",
			GCSCredsSecretName: "foo-test",
			GCSCredsSecretKey:  "bar-test",
		}

		manifest, err = manifest.Transform(populateGoogleCreds(prop))
		assert.NilError(t, err)

		err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
		assert.NilError(t, err)

		path := googleCredsPath + "/" + prop.GCSCredsSecretKey
		newEnv := corev1.EnvVar{
			Name:  googleAPPCredsEnvName,
			Value: path,
		}

		var i int
		for i = range deployment.Spec.Template.Spec.Containers {
			if deployment.Spec.Template.Spec.Containers[i].Name != apiContainerName {
				continue
			}
		}

		assert.Equal(t, deployment.Spec.Template.Spec.Volumes[2].Name, googleCredsVolName)
		assert.Equal(t, deployment.Spec.Template.Spec.Containers[i].VolumeMounts[2].Name, googleCredsVolName)
		assert.Equal(t, deployment.Spec.Template.Spec.Containers[i].Env[5], newEnv)
	}
}

func TestUpdateEnvWithSecretName(t *testing.T) {
	testData := path.Join("testdata", "api-deployment.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	secretName := "my_custom_secret"

	deployment := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)
	prop := v1alpha1.ResultsAPIProperties{
		SecretName: secretName,
	}

	manifest, err = manifest.Transform(updateEnvWithSecretName(prop))
	assert.NilError(t, err)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)

	containerFound := false
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name != apiContainerName {
			continue
		}
		containerFound = true

		// verify properties are present in env slice
		for _, propertyKey := range allowedPropertySecretKeys {
			envFound := false
			for _, _env := range container.Env {
				if _env.Name == propertyKey {
					envFound = true
					assert.Equal(t, propertyKey, _env.ValueFrom.SecretKeyRef.Key)
					assert.Equal(t, secretName, _env.ValueFrom.SecretKeyRef.Name)
					assert.Equal(t, true, *_env.ValueFrom.SecretKeyRef.Optional)
				}
			}
			assert.Equal(t, true, envFound, fmt.Sprintf("property not found in env:%s", propertyKey))
		}
	}
	assert.Equal(t, true, containerFound, "container not found")
}

func TestUpdateAPIEnv(t *testing.T) {
	testData := path.Join("testdata", "api-deployment-env.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	deployment := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)

	boolVal := true
	intVal := int64(12345)
	spec := v1alpha1.TektonResultSpec{
		Result: v1alpha1.Result{

			ResultsAPIProperties: v1alpha1.ResultsAPIProperties{
				DBHost:                "localhost",
				DBName:                "test",
				ServerPort:            &intVal,
				DBEnableAutoMigration: &boolVal,
				TLSHostnameOverride:   "localhostTest",
				AuthDisable:           &boolVal,
				LogLevel:              "warn",
				LogsAPI:               &boolVal,
				LogsPath:              "/logs/test",
				LogsType:              "S3",
				LogsBufferSize:        &intVal,
			},
		},
	}

	manifest, err = manifest.Transform(updateApiEnv(spec))
	assert.NilError(t, err)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)

	containerFound := false
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name != apiContainerName {
			continue
		}
		containerFound = true
		envFound := false
		for _, env := range container.Env {
			switch env.Name {
			case "LOGS_PATH":
				envFound = true
				assert.Equal(t, env.Value, "/logs/test")
			case "LOGS_TYPE":
				envFound = true
				assert.Equal(t, env.Value, "S3")
			case "SERVER_PORT":
				envFound = true
				assert.Equal(t, env.Value, "12345")
			case "AUTH_DISABLE":
				envFound = true
				assert.Equal(t, env.Value, "true")
			}
		}
		assert.Equal(t, true, envFound, "env not found")

	}
	assert.Equal(t, true, containerFound, "container not found")
}

func TestUpdateEnvWithDBSecretName(t *testing.T) {
	testData := path.Join("testdata", "api-deployment.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	dbSecretName := "my_custom_secret"

	deployment := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)
	prop := v1alpha1.ResultsAPIProperties{
		DBSecretName:        dbSecretName,
		DBSecretUserKey:     "user1",
		DBSecretPasswordKey: "random-password",
	}

	manifest, err = manifest.Transform(updateEnvWithDBSecretName(prop))
	assert.NilError(t, err)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)

	containerFound := false
	for _, container := range deployment.Spec.Template.Spec.Containers {
		if container.Name != apiContainerName {
			continue
		}
		containerFound = true
		// verify api container env reference custom db secret key and name
		for envKeyName := range ContainerEnvKeys {
			envFound := false
			for _, _env := range container.Env {
				if _env.Name == envKeyName {
					envFound = true
					assert.Equal(t, dbSecretName, _env.ValueFrom.SecretKeyRef.LocalObjectReference.Name)
				}
				if _env.Name == DB_USER {
					assert.Equal(t, "user1", _env.ValueFrom.SecretKeyRef.Key)
				}
				if _env.Name == DB_PASSWORD {
					assert.Equal(t, "random-password", _env.ValueFrom.SecretKeyRef.Key)
				}

			}
			assert.Equal(t, true, envFound, fmt.Sprintf("secret name %s not found in env:%s", dbSecretName, envKeyName))
		}

	}
	assert.Equal(t, true, containerFound, "container not found")
}

func Test_AddConfiguration(t *testing.T) {
	testData := path.Join("testdata", "api-deployment.yaml")
	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	deployment := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)

	prop := v1alpha1.TektonResultSpec{
		Config: v1alpha1.Config{
			NodeSelector: map[string]string{
				"kubernetes.io/os": "linux",
				"node-type":        "compute",
			},
			Tolerations: []corev1.Toleration{
				{
					Key:      "node.kubernetes.io/not-ready",
					Operator: corev1.TolerationOpExists,
					Effect:   corev1.TaintEffectNoExecute,
				},
			},
			PriorityClassName: "system-cluster-critical",
		},
	}

	// Apply the AddConfiguration transformer
	manifest, err = manifest.Transform(common.AddConfiguration(prop.Config))
	assert.NilError(t, err)

	// Convert back to deployment to verify changes
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(manifest.Resources()[0].Object, deployment)
	assert.NilError(t, err)

	// Verify NodeSelector was applied
	assert.Equal(t, len(deployment.Spec.Template.Spec.NodeSelector), 2)
	assert.Equal(t, deployment.Spec.Template.Spec.NodeSelector["kubernetes.io/os"], "linux")
	assert.Equal(t, deployment.Spec.Template.Spec.NodeSelector["node-type"], "compute")

	// Verify Tolerations were applied
	assert.Equal(t, len(deployment.Spec.Template.Spec.Tolerations), 1)
	assert.Equal(t, deployment.Spec.Template.Spec.Tolerations[0].Key, "node.kubernetes.io/not-ready")
	assert.Equal(t, deployment.Spec.Template.Spec.Tolerations[0].Operator, corev1.TolerationOpExists)
	assert.Equal(t, deployment.Spec.Template.Spec.Tolerations[0].Effect, corev1.TaintEffectNoExecute)

	// Verify PriorityClassName was applied
	assert.Equal(t, deployment.Spec.Template.Spec.PriorityClassName, "system-cluster-critical")
}
