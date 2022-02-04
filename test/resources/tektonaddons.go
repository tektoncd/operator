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

	"github.com/tektoncd/operator/pkg/reconciler/common"

	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"

	"knative.dev/pkg/test/logging"

	"github.com/tektoncd/operator/test/utils"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	addonv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/tektonaddon"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureTektonAddonExists creates a TektonAddon with the name names.TektonAddon, if it does not exist.
func EnsureTektonAddonExists(clients addonv1alpha1.TektonAddonInterface, names utils.ResourceNames) (*v1alpha1.TektonAddon, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	ks, err := clients.Get(context.TODO(), names.TektonAddon, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ks := &v1alpha1.TektonAddon{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonAddon,
			},
			Spec: v1alpha1.TektonAddonSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
			},
		}
		return clients.Create(context.TODO(), ks, metav1.CreateOptions{})
	}
	return ks, err
}

// WaitForTektonAddonState polls the status of the TektonAddon called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForTektonAddonState(clients addonv1alpha1.TektonAddonInterface, name string,
	inState func(s *v1alpha1.TektonAddon, err error) (bool, error)) (*v1alpha1.TektonAddon, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonAddonState/%s/%s", name, "TektonAddonIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonAddon
	waitErr := wait.PollImmediate(Interval, Timeout, func() (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektonaddon %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsTektonAddonReady will check the status conditions of the TektonAddon and return true if the TektonAddon is ready.
func IsTektonAddonReady(s *v1alpha1.TektonAddon, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// AssertTektonAddonCRReadyStatus verifies if the TektonAddon reaches the READY status.
func AssertTektonAddonCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonAddonState(clients.TektonAddon(), names.TektonAddon,
		IsTektonAddonReady); err != nil {
		t.Fatalf("TektonAddonCR %q failed to get to the READY status: %v", names.TektonAddon, err)
	}
}

// AssertTektonInstallerSets verifies if the TektonInstallerSets are created.
func AssertTektonInstallerSets(t *testing.T, clients *utils.Clients) {
	assertInstallerSets(t, clients, tektonaddon.ClusterTaskInstallerSet)
	assertInstallerSets(t, clients, tektonaddon.VersionedClusterTaskInstallerSet)
	assertInstallerSets(t, clients, tektonaddon.PipelinesTemplateInstallerSet)
	assertInstallerSets(t, clients, tektonaddon.TriggersResourcesInstallerSet)
	assertInstallerSets(t, clients, tektonaddon.ConsoleCLIInstallerSet)
	assertInstallerSets(t, clients, tektonaddon.MiscellaneousResourcesInstallerSet)
}

func assertInstallerSets(t *testing.T, clients *utils.Clients, component string) {
	ls := metav1.LabelSelector{
		MatchLabels: map[string]string{
			v1alpha1.InstallerSetType: component,
		},
	}
	labelSelector, err := common.LabelSelector(ls)
	if err != nil {
		t.Fatal(err)
	}
	installerSets, err := clients.TektonInstallerSet().List(context.TODO(), metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		t.Fatalf("failed to get TektonInstallerSet for %s : %v", component, err)
	}
	if len(installerSets.Items) > 1 {
		t.Fatalf("multiple installer sets for %s TektonInstallerSet", component)
	}
}

// TektonAddonCRDelete deletes tha TektonAddon to see if all resources will be deleted
func TektonAddonCRDelete(t *testing.T, clients *utils.Clients, crNames utils.ResourceNames) {
	if err := clients.TektonAddon().Delete(context.TODO(), crNames.TektonAddon, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("TektonAddon %q failed to delete: %v", crNames.TektonAddon, err)
	}
	err := wait.PollImmediate(Interval, Timeout, func() (bool, error) {
		_, err := clients.TektonAddon().Get(context.TODO(), crNames.TektonAddon, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on TektonAddon to delete", err)
	}
	_, b, _, _ := runtime.Caller(0)
	m, err := mfc.NewManifest(filepath.Join((filepath.Dir(b)+"/.."), "manifests/"), clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if err := verifyNoTektonAddonCR(clients); err != nil {
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

func verifyNoTektonAddonCR(clients *utils.Clients) error {
	addons, err := clients.TektonAddonAll().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(addons.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonAddon exists")
	}
	return nil
}
