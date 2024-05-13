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
	"errors"
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	manualapprovalgatev1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/test/utils"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/test/logging"
)

// EnsureManualApprovalGateExists creates a ManualApprovalGate with the name names.ManualApprovalGate, if it does not exist.
func EnsureManualApprovalGateExists(clients manualapprovalgatev1alpha1.ManualApprovalGateInterface, names utils.ResourceNames) (*v1alpha1.ManualApprovalGate, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	ks, err := clients.Get(context.TODO(), names.ManualApprovalGate, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ks := &v1alpha1.ManualApprovalGate{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.ManualApprovalGate,
			},
			Spec: v1alpha1.ManualApprovalGateSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
			},
		}
		return clients.Create(context.TODO(), ks, metav1.CreateOptions{})
	}
	return ks, err
}

// WaitForManualApprovalGateState polls the status of the ManualApprovalGate called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForManualApprovalGateState(clients manualapprovalgatev1alpha1.ManualApprovalGateInterface, name string,
	inState func(s *v1alpha1.ManualApprovalGate, err error) (bool, error)) (*v1alpha1.ManualApprovalGate, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForManualApprovalGateState/%s/%s", name, "ManualApprovalGateIsReady"))
	defer span.End()

	var lastState *v1alpha1.ManualApprovalGate
	waitErr := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("manualapprovalgate %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsManualApprovalGateReady will check the status conditions of the ManualApprovalGate and return true if the ManualApprovalGate is ready.
func IsManualApprovalGateReady(s *v1alpha1.ManualApprovalGate, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// AssertManualApprovalGateCRReadyStatus verifies if the ManualApprovalGate reaches the READY status.
func AssertManualApprovalGateCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForManualApprovalGateState(clients.ManualApprovalGate(), names.ManualApprovalGate,
		IsManualApprovalGateReady); err != nil {
		t.Fatalf("ManualApprovalGateCR %q failed to get to the READY status: %v", names.ManualApprovalGate, err)
	}
}

// AssertTektonInstallerSets verifies if the TektonInstallerSets are created.
func AssertManualApprovalGateInstallerSets(t *testing.T, clients *utils.Clients) {
	assertInstallerSets(t, clients, v1alpha1.ManualApprovalGates)
}

// ManualApprovalGateCRDelete deletes tha ManualApprovalGate to see if all resources will be deleted
func ManualApprovalGateCRDelete(t *testing.T, clients *utils.Clients, crNames utils.ResourceNames) {
	if err := clients.ManualApprovalGate().Delete(context.TODO(), crNames.ManualApprovalGate, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("ManualApprovalGate %q failed to delete: %v", crNames.ManualApprovalGate, err)
	}
	err := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		_, err := clients.ManualApprovalGate().Get(context.TODO(), crNames.ManualApprovalGate, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on ManualApprovalGate to delete", err)
	}
	_, b, _, _ := runtime.Caller(0)
	m, err := mfc.NewManifest(filepath.Join((filepath.Dir(b)+"/.."), "manifests/"), clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if err := verifyNoManualApprovalGateCR(clients); err != nil {
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

func verifyNoManualApprovalGateCR(clients *utils.Clients) error {
	manualapprovalgates, err := clients.ManualApprovalGateAll().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(manualapprovalgates.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any ManualApprovalGate exists")
	}
	return nil
}
