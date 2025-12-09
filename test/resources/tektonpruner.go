/*
Copyright 2025 The Tekton Authors

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

	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonpruner"
	"github.com/tektoncd/pruner/pkg/config"
	"knative.dev/pkg/ptr"

	yaml "sigs.k8s.io/yaml/goyaml.v2"

	mfc "github.com/manifestival/client-go-client"
	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	TektonPrunerv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/test/utils"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/test/logging"
)

// EnsureTektonPrunerExists creates a TektonPruner with the name names.TektonPruner, if it does not exist.
func EnsureTektonPrunerExists(clients TektonPrunerv1alpha1.TektonPrunerInterface, names utils.ResourceNames) (*v1alpha1.TektonPruner, error) {
	ks, err := clients.Get(context.TODO(), names.TektonPruner, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ks := &v1alpha1.TektonPruner{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonPruner,
			},
			Spec: v1alpha1.TektonPrunerSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
				Pruner: v1alpha1.Pruner{
					TektonPrunerConfig: v1alpha1.TektonPrunerConfig{
						GlobalConfig: &config.GlobalConfig{
							PrunerConfig: config.PrunerConfig{
								SuccessfulHistoryLimit: ptr.Int32(12),
								HistoryLimit:           ptr.Int32(45),
							},
						},
					},
				},
			},
		}
		return clients.Create(context.TODO(), ks, metav1.CreateOptions{})
	}
	return ks, err
}

// WaitForTektonPrunerState polls the status of the TektonPruner called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForTektonPrunerState(clients TektonPrunerv1alpha1.TektonPrunerInterface, name string,
	inState func(s *v1alpha1.TektonPruner, err error) (bool, error),
) (*v1alpha1.TektonPruner, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonPrunerState/%s/%s", name, "TektonPrunerIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonPruner
	waitErr := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("TektonPruner %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsTektonPrunerReady will check the status conditions of the TektonPruner and return true if the TektonPruner is ready.
func IsTektonPrunerReady(s *v1alpha1.TektonPruner, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// AssertTektonPrunerCRReadyStatus verifies if the TektonPruner reaches the READY status.
func AssertTektonPrunerCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonPrunerState(clients.TektonPruner(), names.TektonPruner,
		IsTektonPrunerReady); err != nil {
		t.Fatalf("TektonPrunerCR %q failed to get to the READY status: %v", names.TektonPruner, err)
	}
}

// AssertTektonInstallerSets verifies if the TektonInstallerSets are created.
func AssertTektonPrunerInstallerSets(t *testing.T, clients *utils.Clients) {
	assertInstallerSets(t, clients, tektonpruner.PrunerConfigInstallerSet)
}

func AssertConfigMapData(t *testing.T, clients *utils.Clients, pruner *v1alpha1.TektonPruner) {
	targetNamespace := pruner.Spec.TargetNamespace
	cm, err := clients.KubeClient.CoreV1().ConfigMaps(targetNamespace).Get(context.Background(), tektonpruner.PrunerConfigMapName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Failed to get ConfigMap %s in namespace %s: %v", tektonpruner.PrunerConfigMapName, targetNamespace, err)
	}

	expectedConfig, _ := yaml.Marshal(pruner.Spec.TektonPrunerConfig.GlobalConfig)
	actualConfig := cm.Data["global-config"]

	if actualConfig != string(expectedConfig) {
		t.Fatalf("ConfigMap %s in namespace %s does not contain expected global-config data.\nExpected: %s\nGot: %s",
			tektonpruner.PrunerConfigMapName, targetNamespace, expectedConfig, actualConfig)
	}
}

// TektonPrunerCRDelete deletes tha TektonPruner to see if all resources will be deleted
func TektonPrunerCRDelete(t *testing.T, clients *utils.Clients, crNames utils.ResourceNames) {
	if err := clients.TektonPruner().Delete(context.TODO(), crNames.TektonPruner, metav1.DeleteOptions{}); err != nil {
		t.Fatalf("TektonPruner %q failed to delete: %v", crNames.TektonPruner, err)
	}
	err := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		_, err := clients.TektonPruner().Get(context.TODO(), crNames.TektonPruner, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		t.Fatal("Timed out waiting on TektonPruner to delete", err)
	}
	_, b, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("Failed to get caller information")
	}
	m, err := mfc.NewManifest(filepath.Join((filepath.Dir(b)+"/.."), "manifests/"), clients.Config)
	if err != nil {
		t.Fatal("Failed to load manifest", err)
	}
	if err := verifyNoTektonPrunerCR(clients); err != nil {
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

func verifyNoTektonPrunerCR(clients *utils.Clients) error {
	TektonPruners, err := clients.TektonPrunerAll().List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(TektonPruners.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonPruner exists")
	}
	return nil
}
