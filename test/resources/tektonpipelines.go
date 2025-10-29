/*
Copyright 2020 The Tekton Authors

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

package resources

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"

	"knative.dev/pkg/test/logging"

	"github.com/tektoncd/operator/test/utils"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	pipelinev1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureTektonPipelineExists creates a TektonPipeline with the name names.TektonPipeline, if it does not exist.
func EnsureTektonPipelineExists(clients pipelinev1alpha1.TektonPipelineInterface, names utils.ResourceNames) (*v1alpha1.TektonPipeline, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	tpCR, err := clients.Get(context.TODO(), names.TektonPipeline, metav1.GetOptions{})
	if err == nil {
		return tpCR, err
	}
	if apierrs.IsNotFound(err) {
		tpCR = &v1alpha1.TektonPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonPipeline,
			},
			Spec: v1alpha1.TektonPipelineSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
			},
		}
		return clients.Create(context.TODO(), tpCR, metav1.CreateOptions{})
	}
	return tpCR, err
}

// WaitForTektonPipelineState polls the status of the TektonPipeline called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForTektonPipelineState(clients pipelinev1alpha1.TektonPipelineInterface, name string,
	inState func(s *v1alpha1.TektonPipeline, err error) (bool, error)) (*v1alpha1.TektonPipeline, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonPipelineState/%s/%s", name, "TektonPipelineIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonPipeline
	waitErr := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektonpipeline %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsTektonPipelineReady will check the status conditions of the TektonPipeline and return true if the TektonPipeline is ready.
func IsTektonPipelineReady(s *v1alpha1.TektonPipeline, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// AssertTektonPipelineCRReadyStatus verifies if the TektonPipeline reaches the READY status.
func AssertTektonPipelineCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonPipelineState(clients.TektonPipeline(), names.TektonPipeline,
		IsTektonPipelineReady); err != nil {
		t.Fatalf("TektonPipelineCR %q failed to get to the READY status: %v", names.TektonPipeline, err)
	}
}

// TektonPipelineCRDelete deletes tha TektonPipeline to see if all resources will be deleted
func TektonPipelineCRDelete(t *testing.T, clients *utils.Clients, crNames utils.ResourceNames) {
	if err := clients.TektonPipeline().Delete(context.TODO(), crNames.TektonPipeline, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("TektonPipeline %q failed to delete: %v", crNames.TektonPipeline, err)
	}
	err := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		_, err := clients.TektonPipeline().Get(context.TODO(), crNames.TektonPipeline, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on TektonPipeline to delete", err)
	}
	_, b, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get caller information")
	}
	m, err := mfc.NewManifest(filepath.Join((filepath.Dir(b)+"/.."), "manifests/"), clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if err := verifyNoTektonPipelineCR(clients); err != nil {
		t.Fatal(err)
	}

	// verify all but the CRD's and the Namespace are gone
	for _, u := range m.Filter(mf.NoCRDs, mf.Not(mf.ByKind("Namespace"))).Resources() {
		if _, err := m.Client.Get(&u); !apierrs.IsNotFound(err) {
			t.Fatalf("The %s %s failed to be deleted: %v", u.GetKind(), u.GetName(), err)
		}
	}
	// verify all the CRD's remain
	for _, u := range m.Filter(mf.CRDs).Resources() {
		if _, err := m.Client.Get(&u); apierrs.IsNotFound(err) {
			t.Fatalf("The %s CRD was deleted", u.GetName())
		}
	}
}

func verifyNoTektonPipelineCR(clients *utils.Clients) error {
	pipelines, err := clients.TektonPipelineAll().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(pipelines.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonPipeline exists")
	}
	return nil
}

// EnsureTektonPipelineWithStatefulsetExists creates a TektonPipeline with the name names.TektonPipeline, if it does not exist.
func EnsureTektonPipelineWithStatefulsetExists(clients pipelinev1alpha1.TektonPipelineInterface, names utils.ResourceNames) (*v1alpha1.TektonPipeline, error) {
	// If this function is called by the upgrade tests, we only create the custom resource if it does not exist.
	tpCR, err := clients.Get(context.TODO(), names.TektonPipeline, metav1.GetOptions{})
	if err == nil {
		return tpCR, err
	}
	if apierrs.IsNotFound(err) {
		statefulsetOrdinals := true

		tpCR = &v1alpha1.TektonPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonPipeline,
			},
			Spec: v1alpha1.TektonPipelineSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
				Pipeline: v1alpha1.Pipeline{
					PipelineProperties: v1alpha1.PipelineProperties{
						Performance: v1alpha1.PerformanceProperties{
							PerformanceStatefulsetOrdinalsConfig: v1alpha1.PerformanceStatefulsetOrdinalsConfig{
								StatefulsetOrdinals: &statefulsetOrdinals,
							},
						},
					},
				},
			},
		}
		return clients.Create(context.TODO(), tpCR, metav1.CreateOptions{})
	}
	return tpCR, err
}

// EnsureTektonPipelineWithTracingExists creates a TektonPipeline with tracing configuration
func EnsureTektonPipelineWithTracingExists(
	clients pipelinev1alpha1.TektonPipelineInterface,
	names utils.ResourceNames,
	enabled *bool,
	endpoint string,
	credentialsSecret string,
) (*v1alpha1.TektonPipeline, error) {
	tpCR, err := clients.Get(context.TODO(), names.TektonPipeline, metav1.GetOptions{})
	if err == nil {
		return tpCR, err
	}
	if apierrs.IsNotFound(err) {
		tpCR = &v1alpha1.TektonPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonPipeline,
			},
			Spec: v1alpha1.TektonPipelineSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
				Pipeline: v1alpha1.Pipeline{
					PipelineProperties: v1alpha1.PipelineProperties{
						TracingProperties: v1alpha1.TracingProperties{
							Enabled:           enabled,
							Endpoint:          endpoint,
							CredentialsSecret: credentialsSecret,
						},
					},
				},
			},
		}
		return clients.Create(context.TODO(), tpCR, metav1.CreateOptions{})
	}
	return tpCR, err
}

// GetTracingConfigMap fetches the config-tracing ConfigMap from the specified namespace
func GetTracingConfigMap(clients *utils.Clients, namespace string) (map[string]string, error) {
	cm, err := clients.KubeClient.CoreV1().ConfigMaps(namespace).Get(context.Background(), "config-tracing", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cm.Data, nil
}

// AssertTracingConfigMapData verifies config-tracing ConfigMap has expected values
func AssertTracingConfigMapData(
	t *testing.T,
	clients *utils.Clients,
	targetNamespace string,
	expectedData map[string]string,
) {
	cm, err := clients.KubeClient.CoreV1().ConfigMaps(targetNamespace).Get(context.Background(), "config-tracing", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap config-tracing in namespace %s: %v", targetNamespace, err)
	}

	// Verify expected data (excluding _example field)
	for key, expectedValue := range expectedData {
		actualValue, found := cm.Data[key]
		if !found {
			t.Fatalf("Key '%s' not found in config-tracing ConfigMap", key)
		}
		if actualValue != expectedValue {
			t.Fatalf("ConfigMap config-tracing key '%s' has value '%s', expected '%s'", key, actualValue, expectedValue)
		}
	}

	// Verify no unexpected fields (excluding _example)
	for key := range cm.Data {
		if key == "_example" {
			continue
		}
		_, expected := expectedData[key]
		if !expected {
			t.Fatalf("Unexpected field '%s' in config-tracing ConfigMap", key)
		}
	}
}
