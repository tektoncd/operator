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

package chains

import (
	"context"
	"errors"
	"fmt"
	"log"
	"reflect"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"

	"github.com/tektoncd/operator/pkg/reconciler/common"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/test/logging"

	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	operatorv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
)

const tektonChainsNamespace = "tekton-chains"

// CreateChainsCR creates a Tekton Chains CR in tekton-chains namespace
func CreateChainsCR(ctx context.Context, instance v1alpha1.TektonComponent, client operatorv1alpha1.OperatorV1alpha1Interface) error {
	configInstance := instance.(*v1alpha1.TektonConfig)

	// make sure TektonChains CR exists
	if _, err := ensureTektonChainsExists(ctx, client.TektonChainses(), configInstance); err != nil {
		return errors.New(err.Error())
	}

	// wait till TektonChains CR gets becomes "Ready"
	if _, err := waitForTektonChainsState(ctx, client.TektonChainses(), v1alpha1.ChainsResourceName,
		isTektonChainsReady); err != nil {
		log.Println("TektonChains is not in Ready state yet: ", err)
		return err
	}
	return nil
}

func ensureTektonChainsExists(ctx context.Context, clients op.TektonChainsInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonChains, error) {
	tcCR, err := GetChains(ctx, clients, v1alpha1.ChainsResourceName)
	if err != nil {
		// if TektonChains CR is not found in the cluster, then create one
		if apierrs.IsNotFound(err) {
			tcCR = &v1alpha1.TektonChains{
				ObjectMeta: metav1.ObjectMeta{
					Name: v1alpha1.ChainsResourceName,
				},
				Spec: v1alpha1.TektonChainsSpec{
					CommonSpec: v1alpha1.CommonSpec{
						// TektonChains is installed in tekton-chains namespace
						// and not with other components
						TargetNamespace: tektonChainsNamespace,
					},
					Config: config.Spec.Config,
				},
			}
			return clients.Create(ctx, tcCR, metav1.CreateOptions{})
		}
		return nil, err
	}

	// so TektonChains CR does exist in the cluster, checking if any updates are required.
	// if the chains spec is changed then update the instance
	updated := false

	// Chains is installed in tekton-chains namespace, we do not take the target namespace
	// from TektonConfig
	if tcCR.Spec.TargetNamespace != tektonChainsNamespace {
		tcCR.Spec.TargetNamespace = tektonChainsNamespace
		updated = true
	}

	if !reflect.DeepEqual(tcCR.Spec.Config, config.Spec.Config) {
		tcCR.Spec.Config = config.Spec.Config
		updated = true
	}

	if tcCR.ObjectMeta.OwnerReferences == nil {
		ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
		tcCR.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ownerRef}
		updated = true
	}

	if updated {
		return clients.Update(ctx, tcCR, metav1.UpdateOptions{})
	}

	return tcCR, err
}

func GetChains(ctx context.Context, clients op.TektonChainsInterface, name string) (*v1alpha1.TektonChains, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

// waitForTektonChainsState polls the status of the TektonChains called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func waitForTektonChainsState(ctx context.Context, clients op.TektonChainsInterface, name string,
	inState func(s *v1alpha1.TektonChains, err error) (bool, error)) (*v1alpha1.TektonChains, error) {
	span := logging.GetEmitableSpan(ctx, fmt.Sprintf("WaitForTektonChainsState/%s/%s", name, "TektonChainsIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonChains
	waitErr := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		lastState, err := clients.Get(ctx, name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("TektonChains %s is not in desired state, got: %+v: %w: For more info, please check TektonChains CR status", name, lastState, waitErr)
	}
	return lastState, nil
}

// isTektonChainsReady will check the status conditions of the TektonChains and return true if the TektonChains is ready.
func isTektonChainsReady(s *v1alpha1.TektonChains, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// TektonChainsCRDelete deletes TektonChains CR to see if all resources will be deleted
func TektonChainsCRDelete(ctx context.Context, clients op.TektonChainsInterface, name string) error {
	if _, err := GetChains(ctx, clients, v1alpha1.ChainsResourceName); err != nil {
		// nothing to delete if CR does not exist in the cluster
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := clients.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("TektonChains %q failed to delete: %v", name, err)
	}
	err := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		_, err := clients.Get(ctx, name, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("Timed out waiting on TektonChains to delete %v", err)
	}
	return verifyNoTektonChainsCR(ctx, clients)
}

func verifyNoTektonChainsCR(ctx context.Context, clients op.TektonChainsInterface) error {
	chainses, err := clients.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(chainses.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonChains exists")
	}
	return nil
}
