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

package webhook

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	dynamicclient "github.com/tektoncd/operator/pkg/client/injection/dynamic"
	"gotest.tools/v3/assert"
	admissionv1 "k8s.io/api/admission/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	mwhinformer "knative.dev/pkg/client/injection/kube/informers/admissionregistration/v1/mutatingwebhookconfiguration"
	secretinformer "knative.dev/pkg/injection/clients/namespacedkube/informers/core/v1/secret"
	kwebhook "knative.dev/pkg/webhook"
)

func TestDefaultingAdmissionDetectsStandaloneBuilds(t *testing.T) {
	testDefaultingAdmission(t, "Enabled", "Enabled")
}

func TestDefaultingAdmissionDetectsStandaloneCSIWithoutShipwright(t *testing.T) {
	testDefaultingAdmission(t, "Disabled", "Enabled")
}

func TestDefaultingAdmissionDetectsDisabledStandaloneBuilds(t *testing.T) {
	testDefaultingAdmission(t, "Disabled", "Disabled")
}

func testDefaultingAdmission(t *testing.T, shipwrightState, sharedResourceState string) {
	t.Helper()
	t.Setenv("PLATFORM", "openshift")
	t.Setenv("SYSTEM_NAMESPACE", "openshift-operators")
	var lookups atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/apis/operator.openshift.io/v1alpha1/openshiftbuilds/cluster" {
			t.Errorf("unexpected detection request: %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		lookups.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"apiVersion": "operator.openshift.io/v1alpha1",
			"kind":       "OpenShiftBuild",
			"metadata":   map[string]interface{}{"name": "cluster"},
			"spec": map[string]interface{}{
				"shipwright":     map[string]interface{}{"build": map[string]interface{}{"state": shipwrightState}},
				"sharedResource": map[string]interface{}{"state": sharedResourceState},
			},
		})
	}))
	t.Cleanup(server.Close)

	client, err := dynamic.NewForConfig(&rest.Config{Host: server.URL})
	assert.NilError(t, err)
	factory := informers.NewSharedInformerFactory(kubefake.NewSimpleClientset(), 0)
	startupCtx := dynamicclient.WithClient(context.Background(), client)
	startupCtx = context.WithValue(startupCtx, kubeclient.Key{}, kubefake.NewSimpleClientset())
	startupCtx = context.WithValue(startupCtx, mwhinformer.Key{}, factory.Admissionregistration().V1().MutatingWebhookConfigurations())
	startupCtx = context.WithValue(startupCtx, secretinformer.Key{}, factory.Core().V1().Secrets())
	startupCtx = kwebhook.WithOptions(startupCtx, kwebhook.Options{SecretName: "webhook-certs"})
	controller := NewDefaultingAdmissionController(startupCtx, nil)
	t.Cleanup(controller.WorkQueue().ShutDown)
	admission, ok := controller.Reconciler.(kwebhook.AdmissionController)
	assert.Assert(t, ok)

	// Real HTTP admission requests do not inherit the injected startup context.
	requestCtx := context.Background()
	assert.Assert(t, dynamicclient.Get(requestCtx) == nil)
	raw := []byte(`{"apiVersion":"operator.tekton.dev/v1alpha1","kind":"TektonConfig","metadata":{"name":"config"},"spec":{"targetNamespace":"openshift-pipelines","platforms":{"openshift":{}}}}`)
	response := admission.Admit(requestCtx, &admissionv1.AdmissionRequest{
		Operation: admissionv1.Update,
		Kind:      metav1.GroupVersionKind{Group: "operator.tekton.dev", Version: "v1alpha1", Kind: "TektonConfig"},
		Resource:  metav1.GroupVersionResource{Group: "operator.tekton.dev", Version: "v1alpha1", Resource: "tektonconfigs"},
		Object:    runtime.RawExtension{Raw: raw},
		OldObject: runtime.RawExtension{Raw: raw},
	})
	assert.Assert(t, response.Allowed, "admission failed: %+v", response.Result)

	var patches []struct {
		Path  string
		Value json.RawMessage
	}
	assert.NilError(t, json.Unmarshal(response.Patch, &patches))
	for _, component := range []string{"shipwrightBuild", "sharedResource"} {
		found := false
		for _, patch := range patches {
			if patch.Path == "/spec/platforms/openshift/"+component {
				var value struct{ Enabled bool }
				assert.NilError(t, json.Unmarshal(patch.Value, &value))
				assert.Equal(t, value.Enabled, false, "%s must default disabled when OpenShiftBuild/cluster exists", component)
				found = true
			}
		}
		assert.Assert(t, found, "missing defaulting patch for %s", component)
	}
	assert.Assert(t, lookups.Load() > 0, "admission never queried OpenShiftBuild/cluster")
}
