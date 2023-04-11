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
	typedv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/test/utils"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/test/logging"
)

func EnsureTektonHubExists(clients typedv1alpha1.TektonHubInterface, hub *v1alpha1.TektonHub) (*v1alpha1.TektonHub, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	ks, err := clients.Get(context.TODO(), hub.GetName(), metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		return clients.Create(context.TODO(), hub, metav1.CreateOptions{})
	}
	return ks, err
}

// WaitForTektonHubState polls the status of the TektonHub called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForTektonHubState(clients typedv1alpha1.TektonHubInterface, name string,
	inState func(s *v1alpha1.TektonHub, err error) (bool, error)) (*v1alpha1.TektonHub, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonHubState/%s/%s", name, "TektonHubIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonHub
	waitErr := wait.PollImmediate(utils.Interval, utils.Timeout, func() (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektonhub %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}

	return lastState, nil
}

// IsTektonHubReady will check the status conditions of the TektonHub and return true if the TektonHub is ready.
func IsTektonHubReady(s *v1alpha1.TektonHub, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// AssertTektonHubCRReadyStatus verifies if the TektonHub reaches the READY status.
func AssertTektonHubCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonHubState(clients.TektonHub(), names.TektonHub, IsTektonHubReady); err != nil {
		t.Fatalf("TektonHubCR %q failed to get to the READY status: %v", names.TektonHub, err)
	}
}

// TektonHubCRDelete deletes tha TektonHub to see if all resources will be deleted
func TektonHubCRDelete(t *testing.T, clients *utils.Clients, crNames utils.ResourceNames) {
	if err := clients.TektonHub().Delete(context.TODO(), crNames.TektonHub, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			return
		}
		t.Fatalf("TektonHub %q failed to delete: %v", crNames.TektonHub, err)
	}
	err := wait.PollImmediate(utils.Interval, utils.Timeout, func() (bool, error) {
		_, err := clients.TektonHub().Get(context.TODO(), crNames.TektonHub, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on TektonHub to delete", err)
	}
	_, b, _, _ := runtime.Caller(0)
	m, err := mfc.NewManifest(filepath.Join((filepath.Dir(b)+"/.."), "manifests/"), clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if err := verifyNoTektonHubCR(clients); err != nil {
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

func verifyNoTektonHubCR(clients *utils.Clients) error {
	hub, err := clients.TektonHubAll().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(hub.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonHub exists")
	}
	return nil
}
