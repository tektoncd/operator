/*
Copyright 2021 The Tekton Authors

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
	resultv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureTektonResultExists creates a TektonResult with the name names.TektonResult, if it does not exist.
func EnsureTektonResultExists(clients resultv1alpha1.TektonResultInterface, names utils.ResourceNames) (*v1alpha1.TektonResult, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	trCR, err := clients.Get(context.TODO(), names.TektonResult, metav1.GetOptions{})
	if err == nil {
		return trCR, err
	}
	if apierrs.IsNotFound(err) {
		trCR = &v1alpha1.TektonResult{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonResult,
			},
			Spec: v1alpha1.TektonResultSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
			},
		}
		return clients.Create(context.TODO(), trCR, metav1.CreateOptions{})
	}
	return trCR, err
}

// WaitForTektonResultState polls the status of the TektonResult called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForTektonResultState(clients resultv1alpha1.TektonResultInterface, name string,
	inState func(s *v1alpha1.TektonResult, err error) (bool, error)) (*v1alpha1.TektonResult, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonResultState/%s/%s", name, "TektonResultIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonResult
	waitErr := wait.PollImmediate(utils.Interval, utils.Timeout, func() (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektonresult %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsTektonResultReady will check the status conditions of the TektonResult and return true if the TektonResult is ready.
func IsTektonResultReady(s *v1alpha1.TektonResult, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// AssertTektonResultCRReadyStatus verifies if the TektonResult reaches the READY status.
func AssertTektonResultCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonResultState(clients.TektonResult(), names.TektonResult,
		IsTektonResultReady); err != nil {
		t.Fatalf("TektonResultCR %q failed to get to the READY status: %v", names.TektonResult, err)
	}
}

// TektonResultCRDDelete deletes tha TektonResult to see if all resources will be deleted
func TektonResultCRDDelete(t *testing.T, clients *utils.Clients, crNames utils.ResourceNames) {
	if err := clients.TektonResult().Delete(context.TODO(), crNames.TektonResult, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("TektonResult %q failed to delete: %v", crNames.TektonResult, err)
	}
	err := wait.PollImmediate(utils.Interval, utils.Timeout, func() (bool, error) {
		_, err := clients.TektonResult().Get(context.TODO(), crNames.TektonResult, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on TektonResult to delete", err)
	}
	_, b, _, _ := runtime.Caller(0)
	m, err := mfc.NewManifest(filepath.Join(filepath.Dir(b)+"/..", "manifests/"), clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if err := verifyNoTektonResultCR(clients); err != nil {
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

func verifyNoTektonResultCR(clients *utils.Clients) error {
	results, err := clients.TektonResultAll().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(results.Items) > 0 {
		return errors.New("unable to verify cluster-scoped resources are deleted if any TektonResult exists")
	}
	return nil
}
