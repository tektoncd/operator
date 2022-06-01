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
	dashboardv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureTektonDashboardExists creates a TektonDashboard with the name names.TektonDashboard, if it does not exist.
func EnsureTektonDashboardExists(clients dashboardv1alpha1.TektonDashboardInterface, names utils.ResourceNames) (*v1alpha1.TektonDashboard, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	ks, err := clients.Get(context.TODO(), names.TektonDashboard, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ks := &v1alpha1.TektonDashboard{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonDashboard,
			},
			Spec: v1alpha1.TektonDashboardSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
			},
		}
		return clients.Create(context.TODO(), ks, metav1.CreateOptions{})
	}
	return ks, err
}

// WaitForTektonDashboardState polls the status of the TektonDashboard called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForTektonDashboardState(clients dashboardv1alpha1.TektonDashboardInterface, name string,
	inState func(s *v1alpha1.TektonDashboard, err error) (bool, error)) (*v1alpha1.TektonDashboard, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonDashboardState/%s/%s", name, "TektonDashboardIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonDashboard
	waitErr := wait.PollImmediate(utils.Interval, utils.Timeout, func() (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektondashboard %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsTektonDashboardReady will check the status conditions of the TektonDashboard and return true if the TektonDashboard is ready.
func IsTektonDashboardReady(s *v1alpha1.TektonDashboard, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// AssertTektonInstallerSets verifies if the TektonInstallerSets are created.
func AssertDashboardInstallerSets(t *testing.T, clients *utils.Clients) {
	assertInstallerSets(t, clients, v1alpha1.DashboardResourceName)
}

// AssertTektonDashboardCRReadyStatus verifies if the TektonDashboard reaches the READY status.
func AssertTektonDashboardCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonDashboardState(clients.TektonDashboard(), names.TektonDashboard,
		IsTektonDashboardReady); err != nil {
		t.Fatalf("TektonDashboardCR %q failed to get to the READY status: %v", names.TektonDashboard, err)
	}
}

// TektonDashboardCRDelete deletes tha TektonDashboard to see if all resources will be deleted
func TektonDashboardCRDelete(t *testing.T, clients *utils.Clients, crNames utils.ResourceNames) {
	if err := clients.TektonDashboard().Delete(context.TODO(), crNames.TektonDashboard, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("TektonDashboard %q failed to delete: %v", crNames.TektonDashboard, err)
	}
	err := wait.PollImmediate(utils.Interval, utils.Timeout, func() (bool, error) {
		_, err := clients.TektonDashboard().Get(context.TODO(), crNames.TektonDashboard, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on TektonDashboard to delete", err)
	}
	_, b, _, _ := runtime.Caller(0)
	m, err := mfc.NewManifest(filepath.Join((filepath.Dir(b)+"/.."), "manifests/"), clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if err := verifyNoTektonDashboardCR(clients); err != nil {
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

func verifyNoTektonDashboardCR(clients *utils.Clients) error {
	dashboards, err := clients.TektonDashboardAll().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(dashboards.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonDashboard exists")
	}
	return nil
}
