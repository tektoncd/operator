/*
Copyright 2024 The Tekton Authors

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
	"fmt"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	typedv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/test/utils"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/test/logging"
)

// EnsureOpenShiftPipelinesAsCodeExists creates a OpenShiftPipelinesAsCode with the name names.OpenShiftPipelinesAsCode, if it does not exist.
func EnsureOpenShiftPipelinesAsCodeExists(clients typedv1alpha1.OpenShiftPipelinesAsCodeInterface, names utils.ResourceNames) (*v1alpha1.OpenShiftPipelinesAsCode, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	ks, err := clients.Get(context.TODO(), names.OpenShiftPipelinesAsCode, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ks := &v1alpha1.OpenShiftPipelinesAsCode{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.OpenShiftPipelinesAsCode,
			},
			Spec: v1alpha1.OpenShiftPipelinesAsCodeSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
				PACSettings: v1alpha1.PACSettings{
					Settings: map[string]string{},
				},
			},
		}
		return clients.Create(context.TODO(), ks, metav1.CreateOptions{})
	}
	return ks, err
}

// WaitForOpenshiftPipelinesAsCodeState polls the status of the OpenShift Pipelines As Code called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForOpenshiftPipelinesAsCodeState(clients typedv1alpha1.OpenShiftPipelinesAsCodeInterface, name string,
	inState func(s *v1alpha1.OpenShiftPipelinesAsCode, err error) (bool, error)) (*v1alpha1.OpenShiftPipelinesAsCode, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForOpenShiftPipelinesAsCodeState/%s/%s", name, "TektonOpenShiftPipelinesASCodeIsReady"))
	defer span.End()

	var lastState *v1alpha1.OpenShiftPipelinesAsCode
	waitErr := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("OpenShiftPipelinesAsCode %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsOpenShiftPipelinesAsCodeReady will check the status conditions of the OpenShiftPipelinesAsCode and return true if the OpenShiftPipelinesASCode is ready.
func IsOpenShiftPipelinesAsCodeReady(s *v1alpha1.OpenShiftPipelinesAsCode, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// AssertOpenShiftPipelinesCRReadyStatus verifies if the OpenShiftPIpelinesAsCode reaches the READY status.
func AssertOpenShiftPipelinesAsCodeCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForOpenshiftPipelinesAsCodeState(clients.OpenShiftPipelinesAsCode(), names.OpenShiftPipelinesAsCode, IsOpenShiftPipelinesAsCodeReady); err != nil {
		t.Fatalf("OpenShiftPipelinesAsCodeCR %q failed to get to the READY status: %v", names.OpenShiftPipelinesAsCode, err)
	}
}

// Fetch the OpenShiftPipelinesAsCode CR and update the spec to create additional controller of PAC
func CreateAdditionalPipelinesAsCodeController(clients typedv1alpha1.OpenShiftPipelinesAsCodeInterface, names utils.ResourceNames) (*v1alpha1.OpenShiftPipelinesAsCode, error) {
	// fetch the OpenShiftPipelinesAsCode CR
	opacCR, err := clients.Get(context.TODO(), names.OpenShiftPipelinesAsCode, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// update the OpenshiftPipelines CR to add the additional Pipelines As Code Controller
	opacCR.Spec.PACSettings.AdditionalPACControllers = map[string]v1alpha1.AdditionalPACControllerConfig{
		"additional-test": {
			ConfigMapName: "additional-test-configmap",
			SecretName:    "additional-test-secret",
		},
	}
	return clients.Update(context.TODO(), opacCR, metav1.UpdateOptions{})
}

// Fetch the OpenShiftPipelinesAsCode CR and delete the additional pipelines as code config
func RemoveAdditionalPipelinesAsCodeController(clients typedv1alpha1.OpenShiftPipelinesAsCodeInterface, names utils.ResourceNames) (*v1alpha1.OpenShiftPipelinesAsCode, error) {
	// fetch the OpenShiftPipelinesAsCode CR
	opacCR, err := clients.Get(context.TODO(), names.OpenShiftPipelinesAsCode, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// update the OpenshiftPipelines CR to remove the additional Pipelines As Code Controller
	opacCR.Spec.PACSettings.AdditionalPACControllers = map[string]v1alpha1.AdditionalPACControllerConfig{}
	return clients.Update(context.TODO(), opacCR, metav1.UpdateOptions{})
}

// OpenShiftPipelinesASCodeCRDelete deletes the OpenShiftPipelinesAsCode to see if all resources will be deleted
func OpenShiftPipelinesAsCodeCRDelete(t *testing.T, clients *utils.Clients, crNames utils.ResourceNames) {
	if err := clients.OpenShiftPipelinesAsCode().Delete(context.TODO(), crNames.OpenShiftPipelinesAsCode, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("OpenShiftPipelinesAsCode %q failed to delete: %v", crNames.OpenShiftPipelinesAsCode, err)
	}
	err := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		_, err := clients.OpenShiftPipelinesAsCode().Get(context.TODO(), crNames.OpenShiftPipelinesAsCode, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on OpenShiftPipelinesAsCode to delete", err)
	}
}
