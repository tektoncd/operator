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

package resources

import (
	"context"
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
	sharedresourcev1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureSharedResourceExists creates a SharedResource with the name names.SharedResource, if it does not exist.
func EnsureSharedResourceExists(clients sharedresourcev1alpha1.SharedResourceInterface, names utils.ResourceNames) (*v1alpha1.SharedResource, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	sr, err := clients.Get(context.TODO(), names.SharedResource, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		sr := &v1alpha1.SharedResource{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.SharedResource,
			},
			Spec: v1alpha1.SharedResourceSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
			},
		}
		return clients.Create(context.TODO(), sr, metav1.CreateOptions{})
	}
	return sr, err
}

// WaitForSharedResourceState polls the status of the SharedResource called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForSharedResourceState(clients sharedresourcev1alpha1.SharedResourceInterface, name string,
	inState func(s *v1alpha1.SharedResource, err error) (bool, error)) (*v1alpha1.SharedResource, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForSharedResourceState/%s/%s", name, "SharedResourceIsReady"))
	defer span.End()

	var lastState *v1alpha1.SharedResource
	waitErr := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("sharedresource %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsSharedResourceReady will check the status conditions of the SharedResource and return true if the SharedResource is ready.
func IsSharedResourceReady(s *v1alpha1.SharedResource, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// AssertSharedResourceInstallerSets verifies if the TektonInstallerSets are created.
func AssertSharedResourceInstallerSets(t *testing.T, clients *utils.Clients) {
	assertInstallerSets(t, clients, v1alpha1.SharedResourceResourceName)
}

// AssertSharedResourceReadyStatus verifies if the SharedResource reaches the READY status.
func AssertSharedResourceReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForSharedResourceState(clients.SharedResource(), names.SharedResource,
		IsSharedResourceReady); err != nil {
		t.Fatalf("SharedResourceCR %q failed to get to the READY status: %v", names.SharedResource, err)
	}
}

// DeleteSharedResource deletes the SharedResource to see if all resources will be deleted
func DeleteSharedResource(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if err := clients.SharedResource().Delete(context.TODO(), names.SharedResource, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("SharedResource %q failed to delete: %v", names.SharedResource, err)
	}
	err := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		_, err := clients.SharedResource().Get(context.TODO(), names.SharedResource, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on SharedResource to delete", err)
	}
	_, b, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get caller information")
	}
	m, err := mfc.NewManifest(filepath.Join((filepath.Dir(b)+"/.."), "manifests/"), clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if srl, err := clients.SharedResource().List(context.TODO(), metav1.ListOptions{}); err != nil {
		t.Fatal(err)
	} else if len(srl.Items) > 0 {
		t.Fatal("Unable to verify cluster-scoped resources are deleted if any SharedResource exists")
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
