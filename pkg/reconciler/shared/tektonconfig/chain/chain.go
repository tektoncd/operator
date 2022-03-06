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

package chain

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func EnsureTektonChainExists(ctx context.Context, clients op.TektonChainInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonChain, error) {
	tcCR, err := GetChain(ctx, clients, v1alpha1.ChainResourceName)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		// if TektonChain CR is not found in the cluster, then create one
		_, err = CreateChain(ctx, clients, config)
		if err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	// so TektonChain CR does exist in the cluster, checking if any updates are required.
	// if the chain spec is changed then update the instance
	tcCR, err = UpdateChain(ctx, tcCR, config, clients)
	if err != nil {
		return nil, err
	}

	ok, err := isTektonChainReady(tcCR, err)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return tcCR, err
}

func GetChain(ctx context.Context, clients op.TektonChainInterface, name string) (*v1alpha1.TektonChain, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

func CreateChain(ctx context.Context, clients op.TektonChainInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonChain, error) {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
	tcCR := &v1alpha1.TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.ChainResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonChainSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			Config: config.Spec.Config,
		},
	}
	_, err := clients.Create(ctx, tcCR, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return tcCR, err
}

func UpdateChain(ctx context.Context, tcCR *v1alpha1.TektonChain, config *v1alpha1.TektonConfig, clients op.TektonChainInterface) (*v1alpha1.TektonChain, error) {
	// if the chain spec is changed then update the instance
	updated := false

	if config.Spec.TargetNamespace != tcCR.Spec.TargetNamespace {
		tcCR.Spec.TargetNamespace = config.Spec.TargetNamespace
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
		_, err := clients.Update(ctx, tcCR, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}
	return tcCR, nil
}

// isTektonChainReady will check the status conditions of the TektonChain and return true if the TektonChain is ready.
func isTektonChainReady(s *v1alpha1.TektonChain, err error) (bool, error) {
	upgradePending, errInternal := common.CheckUpgradePending(s)
	if err != nil {
		return false, errInternal
	}
	if upgradePending {
		return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
	}
	return s.Status.IsReady(), err
}

// TektonChainCRDelete deletes TektonChain CR to see if all resources will be deleted
func TektonChainCRDelete(ctx context.Context, clients op.TektonChainInterface, name string) error {
	if _, err := GetChain(ctx, clients, v1alpha1.ChainResourceName); err != nil {
		// nothing to delete if CR does not exist in the cluster
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := clients.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("TektonChain %q failed to delete: %v", name, err)
	}
	err := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		_, err := clients.Get(ctx, name, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("Timed out waiting on TektonChain to delete %v", err)
	}
	return verifyNoTektonChainCR(ctx, clients)
}

func verifyNoTektonChainCR(ctx context.Context, clients op.TektonChainInterface) error {
	chains, err := clients.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(chains.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonChain exists")
	}
	return nil
}

func GetTektonConfig() *v1alpha1.TektonConfig {
	return &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile: "all",
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
		},
	}
}
