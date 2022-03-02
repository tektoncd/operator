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

// EnsureTektonChainsExists creates a TektonChains with the name names.TektonChains, if it does not exist.
func EnsureTektonChainsExists(clients typedv1alpha1.TektonChainsInterface, names utils.ResourceNames) (*v1alpha1.TektonChains, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	ks, err := clients.Get(context.TODO(), names.TektonChains, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ks := &v1alpha1.TektonChains{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonChains,
			},
			Spec: v1alpha1.TektonChainsSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
			},
		}
		return clients.Create(context.TODO(), ks, metav1.CreateOptions{})
	}
	return ks, err
}

// WaitForTektonChainsState polls the status of the TektonChains called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForTektonChainsState(clients typedv1alpha1.TektonChainsInterface, name string,
	inState func(s *v1alpha1.TektonChains, err error) (bool, error)) (*v1alpha1.TektonChains, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonChainsState/%s/%s", name, "TektonChainsIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonChains
	waitErr := wait.PollImmediate(Interval, Timeout, func() (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektonchains %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsTektonChainsReady will check the status conditions of the TektonChains and return true if the TektonChains is ready.
func IsTektonChainsReady(s *v1alpha1.TektonChains, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// AssertTektonChainsCRReadyStatus verifies if the TektonChains reaches the READY status.
func AssertTektonChainsCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonChainsState(clients.TektonChains(), names.TektonChains,
		IsTektonChainsReady); err != nil {
		t.Fatalf("TektonChainsCR %q failed to get to the READY status: %v", names.TektonChains, err)
	}
}

// TektonChainsCRDelete deletes tha TektonChains to see if all resources will be deleted
func TektonChainsCRDelete(t *testing.T, clients *utils.Clients, crNames utils.ResourceNames) {
	if err := clients.TektonChains().Delete(context.TODO(), crNames.TektonChains, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("TektonChains %q failed to delete: %v", crNames.TektonChains, err)
	}
	err := wait.PollImmediate(Interval, Timeout, func() (bool, error) {
		_, err := clients.TektonChains().Get(context.TODO(), crNames.TektonChains, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on TektonChains to delete", err)
	}
	_, b, _, _ := runtime.Caller(0)
	m, err := mfc.NewManifest(filepath.Join((filepath.Dir(b)+"/.."), "manifests/"), clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if err := verifyNoTektonChainsCR(clients); err != nil {
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

func verifyNoTektonChainsCR(clients *utils.Clients) error {
	chains, err := clients.TektonChainsAll().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(chains.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonChains exists")
	}
	return nil
}
