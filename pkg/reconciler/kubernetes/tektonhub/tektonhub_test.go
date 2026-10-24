/*
Copyright 2026 The Tekton Authors

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
	"testing"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	"gotest.tools/v3/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestAddSecretKeyValue(t *testing.T) {
	manifest, err := mf.ManifestFrom(mf.Path("testdata/hub/api/api-secret.yaml"))
	assert.NilError(t, err)

	transformed, err := manifest.Transform(addSecretKeyValue(v1alpha1.HubApiSecretName, "REDIRECT_URI", "https://ui.example.com"))
	assert.NilError(t, err)

	data, err := secretDataFromManifest(&transformed, v1alpha1.HubApiSecretName)
	assert.NilError(t, err)
	assert.Equal(t, string(data["REDIRECT_URI"]), "https://ui.example.com")
	assert.Equal(t, string(data["GH_CLIENT_ID"]), "")
	assert.Equal(t, string(data["GLE_URL"]), "")

	// Resources that are not the Hub API secret are left unchanged.
	other := unstructured.Unstructured{}
	other.SetKind("ConfigMap")
	other.SetName(v1alpha1.HubApiSecretName)
	assert.NilError(t, addSecretKeyValue(v1alpha1.HubApiSecretName, "REDIRECT_URI", "https://ui.example.com")(&other))
	_, found, err := unstructured.NestedString(other.Object, "stringData", "REDIRECT_URI")
	assert.NilError(t, err)
	assert.Equal(t, found, false)
}

func TestValidateApiDependenciesAddsRedirectURIOnUpgrade(t *testing.T) {
	ctx := context.Background()
	targetNamespace := "openshift-pipelines"

	secret := upgradeApiSecret(targetNamespace, map[string][]byte{
		"GH_CLIENT_ID":           []byte("user-client-id"),
		"GH_CLIENT_SECRET":       []byte("user-client-secret"),
		"JWT_SIGNING_KEY":        []byte("signing-key"),
		"ACCESS_JWT_EXPIRES_IN":  []byte("30m"),
		"REFRESH_JWT_EXPIRES_IN": []byte("30d"),
		"GHE_URL":                []byte(""),
		"AUTH_BASE_URL":          []byte("https://auth.example.com"),
	})

	kc := kubefake.NewSimpleClientset(secret)
	r := &Reconciler{
		kubeClientSet: kc,
		extension:     common.NoExtension(ctx),
	}
	th := upgradeTektonHub(targetNamespace)
	th.Status.SetUiRoute("https://ui.example.com")

	err := r.validateApiDependencies(ctx, th, "testdata/hub", manifestDirAPI)
	assert.NilError(t, err)

	updated, err := kc.CoreV1().Secrets(targetNamespace).Get(ctx, v1alpha1.HubApiSecretName, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, string(updated.Data["REDIRECT_URI"]), "https://ui.example.com")
	assert.Equal(t, string(updated.Data["GH_CLIENT_ID"]), "user-client-id")
	assert.Equal(t, string(updated.Data["GH_CLIENT_SECRET"]), "user-client-secret")
	assert.Equal(t, string(updated.Data["AUTH_BASE_URL"]), "https://auth.example.com")
	// Keys that exist only in the current manifest are added without
	// replacing credentials already stored on the secret.
	assert.Equal(t, string(updated.Data["GLE_URL"]), "")
}

func TestValidateApiDependenciesKeepsExistingRedirectURI(t *testing.T) {
	ctx := context.Background()
	targetNamespace := "openshift-pipelines"

	secret := upgradeApiSecret(targetNamespace, map[string][]byte{
		"GH_CLIENT_ID":           []byte("user-client-id"),
		"GH_CLIENT_SECRET":       []byte("user-client-secret"),
		"JWT_SIGNING_KEY":        []byte("signing-key"),
		"ACCESS_JWT_EXPIRES_IN":  []byte("30m"),
		"REFRESH_JWT_EXPIRES_IN": []byte("30d"),
		"GHE_URL":                []byte(""),
		"AUTH_BASE_URL":          []byte("https://auth.example.com"),
		"REDIRECT_URI":           []byte("https://custom.example.com"),
	})

	kc := kubefake.NewSimpleClientset(secret)
	r := &Reconciler{
		kubeClientSet: kc,
		extension:     common.NoExtension(ctx),
	}

	err := r.validateApiDependencies(ctx, upgradeTektonHub(targetNamespace), "testdata/hub", manifestDirAPI)
	assert.NilError(t, err)

	updated, err := kc.CoreV1().Secrets(targetNamespace).Get(ctx, v1alpha1.HubApiSecretName, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, string(updated.Data["REDIRECT_URI"]), "https://custom.example.com")
	assert.Equal(t, string(updated.Data["GH_CLIENT_ID"]), "user-client-id")
}

func TestValidateApiDependenciesMissingRequiredKey(t *testing.T) {
	ctx := context.Background()
	targetNamespace := "openshift-pipelines"

	secret := upgradeApiSecret(targetNamespace, map[string][]byte{
		"GH_CLIENT_SECRET": []byte("user-client-secret"),
	})

	kc := kubefake.NewSimpleClientset(secret)
	r := &Reconciler{
		kubeClientSet: kc,
		extension:     common.NoExtension(ctx),
	}

	err := r.validateApiDependencies(ctx, upgradeTektonHub(targetNamespace), "testdata/hub", manifestDirAPI)
	assert.Error(t, err, errKeyMissing.Error())

	updated, getErr := kc.CoreV1().Secrets(targetNamespace).Get(ctx, v1alpha1.HubApiSecretName, metav1.GetOptions{})
	assert.NilError(t, getErr)
	_, found := updated.Data["REDIRECT_URI"]
	assert.Equal(t, found, false)
}

func upgradeTektonHub(targetNamespace string) *v1alpha1.TektonHub {
	return &v1alpha1.TektonHub{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.HubResourceName,
		},
		Spec: v1alpha1.TektonHubSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: targetNamespace,
			},
			Api: v1alpha1.ApiSpec{
				ApiSecretName: v1alpha1.HubApiSecretName,
			},
		},
	}
}

func upgradeApiSecret(targetNamespace string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      v1alpha1.HubApiSecretName,
			Namespace: targetNamespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
}
