/*
Copyright 2023 The Tekton Authors

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
	"fmt"
	"reflect"
	"strings"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func EnsureTektonChainExists(ctx context.Context, clients op.TektonChainInterface, tc *v1alpha1.TektonChain) (*v1alpha1.TektonChain, error) {
	tcCR, err := GetChain(ctx, clients, v1alpha1.ChainResourceName)

	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		if err := CreateChain(ctx, clients, tc); err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	tcCR, err = UpdateChain(ctx, tcCR, tc, clients)
	if err != nil {
		return nil, err
	}

	ready, err := isTektonChainReady(tcCR)
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return tcCR, err
}

func EnsureTektonChainCRNotExists(ctx context.Context, clients op.TektonChainInterface) error {
	if _, err := GetChain(ctx, clients, v1alpha1.ChainResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonChain CR is gone, hence return nil
			return nil
		}
		return err
	}
	// if the Get was successful, try deleting the CR
	if err := clients.Delete(ctx, v1alpha1.ChainResourceName, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonChain CR is gone, hence return nil
			return nil
		}
		return fmt.Errorf("TektonChain %q failed to delete: %v", v1alpha1.ChainResourceName, err)
	}
	// if the Delete API call was success,
	// then return requeue_event
	// so that in a subsequent reconcile call the absence of the CR is verified by one of the 2 checks above
	return v1alpha1.RECONCILE_AGAIN_ERR
}

func GetChain(ctx context.Context, clients op.TektonChainInterface, name string) (*v1alpha1.TektonChain, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

func CreateChain(ctx context.Context, clients op.TektonChainInterface, tt *v1alpha1.TektonChain) error {
	_, err := clients.Create(ctx, tt, metav1.CreateOptions{})
	return err
}

func UpdateChain(ctx context.Context, old *v1alpha1.TektonChain, new *v1alpha1.TektonChain, clients op.TektonChainInterface) (*v1alpha1.TektonChain, error) {
	// if the chain spec is changed then update the instance
	updated := false

	if new.Spec.TargetNamespace != old.Spec.TargetNamespace {
		old.Spec.TargetNamespace = new.Spec.TargetNamespace
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.Chain, new.Spec.Chain) {
		old.Spec.Chain = new.Spec.Chain
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.Config, new.Spec.Config) {
		old.Spec.Config = new.Spec.Config
		updated = true
	}

	if old.ObjectMeta.OwnerReferences == nil {
		old.ObjectMeta.OwnerReferences = new.ObjectMeta.OwnerReferences
		updated = true
	}

	if updated {
		_, err := clients.Update(ctx, old, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}
	return old, nil
}

// isTektonChainReady will check the status conditions of the TektonChain and return true if the TektonChain is ready.
func isTektonChainReady(s *v1alpha1.TektonChain) (bool, error) {
	if s.GetStatus() != nil && s.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(s.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return s.Status.IsReady(), nil
}

func GetTektonChainCR(config *v1alpha1.TektonConfig) *v1alpha1.TektonChain {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
	return &v1alpha1.TektonChain{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.ChainResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonChainSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			Config: config.Spec.Config,
			Chain:  config.Spec.Chain,
		},
	}
}
