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

package trigger

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

func EnsureTektonTriggerExists(ctx context.Context, clients op.TektonTriggerInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonTrigger, error) {
	ttCR, err := GetTrigger(ctx, clients, v1alpha1.TriggerResourceName)

	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		_, err = CreateTrigger(ctx, clients, config)
		if err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	ttCR, err = UpdateTrigger(ctx, ttCR, config, clients)
	if err != nil {
		return nil, err
	}

	ok, err := isTektonTriggerReady(ttCR, err)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return ttCR, err
}

func GetTrigger(ctx context.Context, clients op.TektonTriggerInterface, name string) (*v1alpha1.TektonTrigger, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

func CreateTrigger(ctx context.Context, clients op.TektonTriggerInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonTrigger, error) {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())

	ttCR := &v1alpha1.TektonTrigger{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.TriggerResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonTriggerSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			Config:  config.Spec.Config,
			Trigger: config.Spec.Trigger,
		},
	}
	_, err := clients.Create(ctx, ttCR, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return ttCR, err
}

func UpdateTrigger(ctx context.Context, ttCR *v1alpha1.TektonTrigger, config *v1alpha1.TektonConfig, clients op.TektonTriggerInterface) (*v1alpha1.TektonTrigger, error) {
	// if the trigger spec is changed then update the instance
	updated := false

	if config.Spec.TargetNamespace != ttCR.Spec.TargetNamespace {
		ttCR.Spec.TargetNamespace = config.Spec.TargetNamespace
		updated = true
	}

	if !reflect.DeepEqual(ttCR.Spec.Trigger, config.Spec.Trigger) {
		ttCR.Spec.Trigger = config.Spec.Trigger
		updated = true
	}

	if !reflect.DeepEqual(ttCR.Spec.Config, config.Spec.Config) {
		ttCR.Spec.Config = config.Spec.Config
		updated = true
	}

	if ttCR.ObjectMeta.OwnerReferences == nil {
		ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
		ttCR.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ownerRef}
		updated = true
	}

	if updated {
		_, err := clients.Update(ctx, ttCR, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}
	return ttCR, nil
}

// isTektonTriggerReady will check the status conditions of the TektonTrigger and return true if the TektonTrigger is ready.
func isTektonTriggerReady(s *v1alpha1.TektonTrigger, err error) (bool, error) {
	upgradePending, errInternal := common.CheckUpgradePending(s)
	if err != nil {
		return false, errInternal
	}
	if upgradePending {
		return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
	}
	return s.Status.IsReady(), err
}

// TektonTriggerCRDelete deletes tha TektonTrigger to see if all resources will be deleted
func TektonTriggerCRDelete(ctx context.Context, clients op.TektonTriggerInterface, name string) error {
	if _, err := GetTrigger(ctx, clients, v1alpha1.TriggerResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := clients.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("TektonTrigger %q failed to delete: %v", name, err)
	}
	err := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		_, err := clients.Get(ctx, name, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("Timed out waiting on TektonTrigger to delete %v", err)
	}
	return verifyNoTektonTriggerCR(ctx, clients)
}

func verifyNoTektonTriggerCR(ctx context.Context, clients op.TektonTriggerInterface) error {
	triggers, err := clients.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(triggers.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonTrigger exists")
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
