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
	triggerv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureTektonTriggerExists creates a TektonTrigger with the name names.TektonTrigger, if it does not exist.
func EnsureTektonTriggerExists(clients triggerv1alpha1.TektonTriggerInterface, names utils.ResourceNames) (*v1alpha1.TektonTrigger, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	ks, err := clients.Get(context.TODO(), names.TektonTrigger, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ks := &v1alpha1.TektonTrigger{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonTrigger,
			},
			Spec: v1alpha1.TektonTriggerSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
			},
		}
		return clients.Create(context.TODO(), ks, metav1.CreateOptions{})
	}
	return ks, err
}

// WaitForTektonTriggerState polls the status of the TektonTrigger called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForTektonTriggerState(clients triggerv1alpha1.TektonTriggerInterface, name string,
	inState func(s *v1alpha1.TektonTrigger, err error) (bool, error)) (*v1alpha1.TektonTrigger, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonTriggerState/%s/%s", name, "TektonTriggerIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonTrigger
	waitErr := wait.PollImmediate(utils.Interval, utils.Timeout, func() (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektontriggers %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsTektonTriggerReady will check the status conditions of the TektonTrigger and return true if the TektonTrigger is ready.
func IsTektonTriggerReady(s *v1alpha1.TektonTrigger, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// AssertTektonTriggerCRReadyStatus verifies if the TektonTrigger reaches the READY status.
func AssertTektonTriggerCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonTriggerState(clients.TektonTrigger(), names.TektonTrigger,
		IsTektonTriggerReady); err != nil {
		t.Fatalf("TektonTriggerCR %q failed to get to the READY status: %v", names.TektonTrigger, err)
	}
}

// TektonTriggerCRDelete deletes tha TektonTrigger to see if all resources will be deleted
func TektonTriggerCRDelete(t *testing.T, clients *utils.Clients, crNames utils.ResourceNames) {
	if err := clients.TektonTrigger().Delete(context.TODO(), crNames.TektonTrigger, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("TektonTrigger %q failed to delete: %v", crNames.TektonTrigger, err)
	}
	err := wait.PollImmediate(utils.Interval, utils.Timeout, func() (bool, error) {
		_, err := clients.TektonTrigger().Get(context.TODO(), crNames.TektonTrigger, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on TektonTrigger to delete", err)
	}
	_, b, _, _ := runtime.Caller(0)
	m, err := mfc.NewManifest(filepath.Join((filepath.Dir(b)+"/.."), "manifests/"), clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if err := verifyNoTektonTriggerCR(clients); err != nil {
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

func verifyNoTektonTriggerCR(clients *utils.Clients) error {
	triggers, err := clients.TektonTriggerAll().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(triggers.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonTrigger exists")
	}
	return nil
}
