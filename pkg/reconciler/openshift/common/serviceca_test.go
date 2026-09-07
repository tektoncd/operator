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

package common

import (
	"testing"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	"gotest.tools/v3/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// resource is a small helper that builds an unstructured object with the given
// kind, name and (optional) starting annotations.
func resource(kind, name string, annotations map[string]interface{}) *unstructured.Unstructured {
	metadata := map[string]interface{}{"name": name}
	if annotations != nil {
		metadata["annotations"] = annotations
	}
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"kind":     kind,
		"metadata": metadata,
	}}
}

func TestAddServiceCABundle(t *testing.T) {
	const (
		kind    = "MutatingWebhookConfiguration"
		webhook = "shipwright-build-webhook"
	)

	t.Run("annotates a matching resource", func(t *testing.T) {
		u := resource(kind, webhook, nil)
		err := AddServiceCABundle(kind, []string{webhook})(u)
		assert.NilError(t, err)
		assert.Equal(t, u.GetAnnotations()[InjectCABundleAnnotation], "true")
	})

	t.Run("skips a resource of a different kind", func(t *testing.T) {
		u := resource("Service", webhook, nil)
		err := AddServiceCABundle(kind, []string{webhook})(u)
		assert.NilError(t, err)
		_, ok := u.GetAnnotations()[InjectCABundleAnnotation]
		assert.Equal(t, ok, false)
	})

	t.Run("skips a resource whose name is not listed", func(t *testing.T) {
		u := resource(kind, "some-other-webhook", nil)
		err := AddServiceCABundle(kind, []string{webhook})(u)
		assert.NilError(t, err)
		_, ok := u.GetAnnotations()[InjectCABundleAnnotation]
		assert.Equal(t, ok, false)
	})

	t.Run("preserves existing annotations", func(t *testing.T) {
		u := resource(kind, webhook, map[string]interface{}{"existing": "value"})
		err := AddServiceCABundle(kind, []string{webhook})(u)
		assert.NilError(t, err)
		assert.Equal(t, u.GetAnnotations()["existing"], "value")
		assert.Equal(t, u.GetAnnotations()[InjectCABundleAnnotation], "true")
	})

	t.Run("annotates all resources of the kind when resources is nil", func(t *testing.T) {
		u := resource(kind, "any-webhook", nil)
		err := AddServiceCABundle(kind, nil)(u)
		assert.NilError(t, err)
		assert.Equal(t, u.GetAnnotations()[InjectCABundleAnnotation], "true")
	})

	t.Run("annotates all resources of the kind when resources is empty", func(t *testing.T) {
		u := resource(kind, "any-webhook", nil)
		err := AddServiceCABundle(kind, []string{})(u)
		assert.NilError(t, err)
		assert.Equal(t, u.GetAnnotations()[InjectCABundleAnnotation], "true")
	})

	t.Run("skips a resource of a different kind even when resources is nil", func(t *testing.T) {
		u := resource("Service", webhook, nil)
		err := AddServiceCABundle(kind, nil)(u)
		assert.NilError(t, err)
		_, ok := u.GetAnnotations()[InjectCABundleAnnotation]
		assert.Equal(t, ok, false)
	})
}

func TestAddServiceCertificate(t *testing.T) {
	const (
		serviceName = "shipwright-build-webhook"
		secretName  = "shipwright-build-webhook-cert"
	)

	t.Run("annotates the matching Service", func(t *testing.T) {
		u := resource(common.KindService, serviceName, nil)
		err := AddServiceCertificate(serviceName, secretName)(u)
		assert.NilError(t, err)
		assert.Equal(t, u.GetAnnotations()[ServiceCertificateAnnotation], secretName)
	})

	t.Run("skips a non-Service resource", func(t *testing.T) {
		u := resource("Deployment", serviceName, nil)
		err := AddServiceCertificate(serviceName, secretName)(u)
		assert.NilError(t, err)
		_, ok := u.GetAnnotations()[ServiceCertificateAnnotation]
		assert.Equal(t, ok, false)
	})

	t.Run("skips a Service with a different name", func(t *testing.T) {
		u := resource(common.KindService, "some-other-service", nil)
		err := AddServiceCertificate(serviceName, secretName)(u)
		assert.NilError(t, err)
		_, ok := u.GetAnnotations()[ServiceCertificateAnnotation]
		assert.Equal(t, ok, false)
	})

	t.Run("preserves existing annotations", func(t *testing.T) {
		u := resource(common.KindService, serviceName, map[string]interface{}{"existing": "value"})
		err := AddServiceCertificate(serviceName, secretName)(u)
		assert.NilError(t, err)
		assert.Equal(t, u.GetAnnotations()["existing"], "value")
		assert.Equal(t, u.GetAnnotations()[ServiceCertificateAnnotation], secretName)
	})

	t.Run("errors when secretName is empty", func(t *testing.T) {
		u := resource(common.KindService, serviceName, nil)
		err := AddServiceCertificate(serviceName, "")(u)
		assert.ErrorContains(t, err, "secretName and serviceName must not be empty")
	})

	t.Run("errors when serviceName is empty", func(t *testing.T) {
		u := resource(common.KindService, serviceName, nil)
		err := AddServiceCertificate("", secretName)(u)
		assert.ErrorContains(t, err, "secretName and serviceName must not be empty")
	})
}
