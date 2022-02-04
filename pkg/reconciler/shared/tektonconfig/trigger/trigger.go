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

func CreateTriggerCR(ctx context.Context, instance v1alpha1.TektonComponent, client operatorv1alpha1.OperatorV1alpha1Interface) error {
	configInstance := instance.(*v1alpha1.TektonConfig)
	if _, err := ensureTektonTriggerExists(ctx, client.TektonTriggers(), configInstance); err != nil {
		return errors.New(err.Error())
	}
	if _, err := waitForTektonTriggerState(ctx, client.TektonTriggers(), v1alpha1.TriggerResourceName,
		isTektonTriggerReady); err != nil {
		log.Println("TektonTrigger is not in ready state: ", err)
		return err
	}
	return nil
}

func ensureTektonTriggerExists(ctx context.Context, clients op.TektonTriggerInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonTrigger, error) {
	ttCR, err := GetTrigger(ctx, clients, v1alpha1.TriggerResourceName)
	if err == nil {
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
			return clients.Update(ctx, ttCR, metav1.UpdateOptions{})
		}

		return ttCR, err
	}

	if apierrs.IsNotFound(err) {
		ttCR = &v1alpha1.TektonTrigger{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1alpha1.TriggerResourceName,
			},
			Spec: v1alpha1.TektonTriggerSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: config.Spec.TargetNamespace,
				},
				Config:  config.Spec.Config,
				Trigger: config.Spec.Trigger,
			},
		}
		return clients.Create(ctx, ttCR, metav1.CreateOptions{})
	}
	return ttCR, err
}

func GetTrigger(ctx context.Context, clients op.TektonTriggerInterface, name string) (*v1alpha1.TektonTrigger, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

// waitForTektonTriggerState polls the status of the TektonTrigger called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func waitForTektonTriggerState(ctx context.Context, clients op.TektonTriggerInterface, name string,
	inState func(s *v1alpha1.TektonTrigger, err error) (bool, error)) (*v1alpha1.TektonTrigger, error) {
	span := logging.GetEmitableSpan(ctx, fmt.Sprintf("WaitForTektonTriggerState/%s/%s", name, "TektonTriggerIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonTrigger
	waitErr := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		lastState, err := clients.Get(ctx, name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektontrigger %s is not in desired state, got: %+v: %w: For more info Please check TektonTrigger CR status", name, lastState, waitErr)
	}
	return lastState, nil
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
