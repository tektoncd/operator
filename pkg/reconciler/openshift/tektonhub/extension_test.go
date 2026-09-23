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
	"context"
	"path"
	"strings"
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
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

	// Verify PGDATA environment variable value is updated
	var pgdataValue string
	for _, e := range env {
		if e.Name == "PGDATA" {
			pgdataValue = e.Value
			break
		}
	}
	assert.Equal(t, pgdataValue, "/var/lib/pgsql/data")

	mountPath := d.Spec.Template.Spec.Containers[0].VolumeMounts[0].MountPath
	assert.Equal(t, mountPath, "/var/lib/pgsql/data")

	cmd := d.Spec.Template.Spec.Containers[0].ReadinessProbe.Exec.Command
	assert.Equal(t, strings.Contains(cmd[2], "POSTGRESQL_USER"), true)
}

func TestInjectPostgresUpgradeSupport(t *testing.T) {
	testData := path.Join("testdata", "update-db-deployment.yaml")

	manifest, err := mf.ManifestFrom(mf.Recursive(testData))
	assert.NilError(t, err)

	newManifest, err := manifest.Transform(injectPostgresUpgradeSupport())
	assert.NilError(t, err)

	d := &appsv1.Deployment{}
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(newManifest.Resources()[0].Object, d)
	assert.NilError(t, err)

	// Verify command is set to use the wrapper script
	command := d.Spec.Template.Spec.Containers[0].Command
	assert.Equal(t, len(command), 2)
	assert.Equal(t, command[0], "/bin/bash")
	assert.Equal(t, command[1], "/upgrade-scripts/postgres-wrapper.sh")

	// Verify volume mount for upgrade scripts is added
	volumeMounts := d.Spec.Template.Spec.Containers[0].VolumeMounts
	var upgradeScriptsMountFound bool
	for _, vm := range volumeMounts {
		if vm.Name == "upgrade-scripts" {
			upgradeScriptsMountFound = true
			assert.Equal(t, vm.MountPath, "/upgrade-scripts")
			break
		}
	}
	assert.Equal(t, upgradeScriptsMountFound, true)

	// Verify volume for upgrade scripts ConfigMap is added
	volumes := d.Spec.Template.Spec.Volumes
	var upgradeScriptsVolumeFound bool
	for _, vol := range volumes {
		if vol.Name == "upgrade-scripts" {
			upgradeScriptsVolumeFound = true
			assert.Equal(t, vol.ConfigMap.Name, "tekton-hub-postgres-upgrade-scripts")
			assert.Equal(t, *vol.ConfigMap.DefaultMode, int32(0755))
			break
		}
	}
	assert.Equal(t, upgradeScriptsVolumeFound, true)
}

func TestSetAuthAndREDIRECTURIBaseURL(t *testing.T) {
	ctx := context.Background()
	targetNamespace := "tekton-hub"

	t.Run("sets REDIRECT_URI when missing", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tekton-hub-api",
				Namespace: targetNamespace,
			},
			Data: map[string][]byte{
				"GH_CLIENT_ID": []byte("test-client-id"),
			},
		}

		kc := kubefake.NewClientset(secret)
		oe := openshiftExtension{
			kubeClientSet: kc,
		}

		th := &v1alpha1.TektonHub{
			Spec: v1alpha1.TektonHubSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: targetNamespace,
				},
			},
		}
		th.Status.SetUiRoute("https://ui.example.com")
		th.Status.SetAuthRoute("https://auth.example.com")

		err := oe.SetAuthAndREDIRECTURIBaseURL(ctx, th)
		assert.NilError(t, err)

		// Verify REDIRECT_URI was set in StringData
		updatedSecret, err := kc.CoreV1().Secrets(targetNamespace).Get(ctx, "tekton-hub-api", metav1.GetOptions{})
		assert.NilError(t, err)
		assert.Equal(t, updatedSecret.StringData["REDIRECT_URI"], "https://ui.example.com")
		assert.Equal(t, updatedSecret.StringData["AUTH_BASE_URL"], "https://auth.example.com")
	})

	t.Run("updates REDIRECT_URI when changed", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tekton-hub-api",
				Namespace: targetNamespace,
			},
			Data: map[string][]byte{
				"GH_CLIENT_ID": []byte("test-client-id"),
				"REDIRECT_URI": []byte("https://old-ui.example.com"),
			},
		}

		kc := kubefake.NewClientset(secret)
		oe := openshiftExtension{
			kubeClientSet: kc,
		}

		th := &v1alpha1.TektonHub{
			Spec: v1alpha1.TektonHubSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: targetNamespace,
				},
			},
		}
		th.Status.SetUiRoute("https://new-ui.example.com")
		th.Status.SetAuthRoute("https://auth.example.com")

		err := oe.SetAuthAndREDIRECTURIBaseURL(ctx, th)
		assert.NilError(t, err)

		// Verify REDIRECT_URI was updated
		updatedSecret, err := kc.CoreV1().Secrets(targetNamespace).Get(ctx, "tekton-hub-api", metav1.GetOptions{})
		assert.NilError(t, err)
		assert.Equal(t, updatedSecret.StringData["REDIRECT_URI"], "https://new-ui.example.com")
	})

	t.Run("skips update when REDIRECT_URI is already correct", func(t *testing.T) {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "tekton-hub-api",
				Namespace: targetNamespace,
			},
			Data: map[string][]byte{
				"GH_CLIENT_ID":  []byte("test-client-id"),
				"AUTH_BASE_URL": []byte("https://auth.example.com"),
				"REDIRECT_URI":  []byte("https://ui.example.com"),
			},
		}

		kc := kubefake.NewClientset(secret)
		oe := openshiftExtension{
			kubeClientSet: kc,
		}

		th := &v1alpha1.TektonHub{
			Spec: v1alpha1.TektonHubSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: targetNamespace,
				},
			},
		}
		th.Status.SetUiRoute("https://ui.example.com")
		th.Status.SetAuthRoute("https://auth.example.com")

		err := oe.SetAuthAndREDIRECTURIBaseURL(ctx, th)
		assert.NilError(t, err)

		// Verify no update occurred - StringData should be nil or empty
		updatedSecret, err := kc.CoreV1().Secrets(targetNamespace).Get(ctx, "tekton-hub-api", metav1.GetOptions{})
		assert.NilError(t, err)
		// When no update is needed, StringData won't be set
		assert.Equal(t, len(updatedSecret.StringData), 0)
	})
}
