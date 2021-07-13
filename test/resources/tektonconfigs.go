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
	"github.com/tektoncd/operator/pkg/reconciler/common"

	"knative.dev/pkg/test/logging"

	"github.com/tektoncd/operator/test/utils"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	configv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureTektonConfigExists creates a TektonConfig with the name names.TektonConfig, if it does not exist.
func EnsureTektonConfigExists(kubeClientSet *kubernetes.Clientset, clients configv1alpha1.TektonConfigInterface, names utils.ResourceNames) (*v1alpha1.TektonConfig, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.

	cm, err := kubeClientSet.CoreV1().ConfigMaps(names.Namespace).Get(context.TODO(), "tekton-config-defaults", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	tcCR, err := clients.Get(context.TODO(), names.TektonConfig, metav1.GetOptions{})

	if cm.Data["AUTOINSTALL_COMPONENTS"] == "true" {
		if err != nil {
			return nil, err
		}
		return tcCR, nil
	}

	if apierrs.IsNotFound(err) {
		tcCR = &v1alpha1.TektonConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonConfig,
			},
			Spec: v1alpha1.TektonConfigSpec{
				Profile: common.ProfileAll,
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: cm.Data["DEFAULT_TARGET_NAMESPACE"],
				},
				Addon: v1alpha1.Addon{
					Params: []v1alpha1.Param{
						{
							Name:  "pipelineTemplates",
							Value: "true",
						},
						{
							Name:  "clusterTasks",
							Value: "true",
						},
					},
				},
			},
		}
		return clients.Create(context.TODO(), tcCR, metav1.CreateOptions{})
	}
	return tcCR, err
}

// WaitForTektonConfigState polls the status of the TektonConfig called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForTektonConfigState(clients configv1alpha1.TektonConfigInterface, name string,
	inState func(s *v1alpha1.TektonConfig, err error) (bool, error)) (*v1alpha1.TektonConfig, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonConfigState/%s/%s", name, "TektonConfigIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonConfig
	waitErr := wait.PollImmediate(Interval, Timeout, func() (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektonconfig %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsTektonConfigReady will check the status conditions of the TektonConfig and return true if the TektonConfig is ready.
func IsTektonConfigReady(s *v1alpha1.TektonConfig, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// AssertTektonConfigCRReadyStatus verifies if the TektonConfig reaches the READY status.
func AssertTektonConfigCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonConfigState(clients.TektonConfig(), names.TektonConfig,
		IsTektonConfigReady); err != nil {
		t.Fatalf("TektonConfigCR %q failed to get to the READY status: %v", names.TektonConfig, err)
	}
}

// TektonConfigCRDelete deletes tha TektonConfig to see if all resources will be deleted
func TektonConfigCRDelete(t *testing.T, clients *utils.Clients, crNames utils.ResourceNames) {
	if err := clients.TektonConfig().Delete(context.TODO(), crNames.TektonConfig, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("TektonConfig %q failed to delete: %v", crNames.TektonConfig, err)
	}
	err := wait.PollImmediate(Interval, Timeout, func() (bool, error) {
		_, err := clients.TektonConfig().Get(context.TODO(), crNames.TektonConfig, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on TektonConfig to delete", err)
	}
	_, b, _, _ := runtime.Caller(0)
	m, err := mfc.NewManifest(filepath.Join((filepath.Dir(b)+"/.."), "manifests/"), clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if err := verifyNoTektonConfigCR(clients); err != nil {
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

func verifyNoTektonConfigCR(clients *utils.Clients) error {
	configs, err := clients.TektonConfigAll().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(configs.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonConfig exists")
	}
	return nil
}
