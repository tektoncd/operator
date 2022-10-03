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
	"fmt"
	"reflect"
	"strings"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func EnsureTektonTriggerExists(ctx context.Context, clients op.TektonTriggerInterface, tt *v1alpha1.TektonTrigger) (*v1alpha1.TektonTrigger, error) {
	ttCR, err := GetTrigger(ctx, clients, v1alpha1.TriggerResourceName)

	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		if err := CreateTrigger(ctx, clients, tt); err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	ttCR, err = UpdateTrigger(ctx, ttCR, tt, clients)
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

func GetTektonTriggerCR(config *v1alpha1.TektonConfig) *v1alpha1.TektonTrigger {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
	return &v1alpha1.TektonTrigger{
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
}

func CreateTrigger(ctx context.Context, clients op.TektonTriggerInterface, tt *v1alpha1.TektonTrigger) error {
	_, err := clients.Create(ctx, tt, metav1.CreateOptions{})
	return err
}

func UpdateTrigger(ctx context.Context, old *v1alpha1.TektonTrigger, new *v1alpha1.TektonTrigger, clients op.TektonTriggerInterface) (*v1alpha1.TektonTrigger, error) {
	// if the trigger spec is changed then update the instance
	updated := false

	if new.Spec.TargetNamespace != old.Spec.TargetNamespace {
		old.Spec.TargetNamespace = new.Spec.TargetNamespace
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.Trigger, new.Spec.Trigger) {
		old.Spec.Trigger = new.Spec.Trigger
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

// isTektonTriggerReady will check the status conditions of the TektonTrigger and return true if the TektonTrigger is ready.
func isTektonTriggerReady(s *v1alpha1.TektonTrigger, err error) (bool, error) {
	if s.GetStatus() != nil && s.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(s.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return s.Status.IsReady(), err
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

func EnsureTektonTriggerCRNotExists(ctx context.Context, clients op.TektonTriggerInterface) error {
	if _, err := GetTrigger(ctx, clients, v1alpha1.TriggerResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonTrigger CR is gone, hence return nil
			return nil
		}
		return err
	}
	// if the Get was successful, try deleting the CR
	if err := clients.Delete(ctx, v1alpha1.TriggerResourceName, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonTrigger CR is gone, hence return nil
			return nil
		}
		return fmt.Errorf("TektonTrigger %q failed to delete: %v", v1alpha1.TriggerResourceName, err)
	}
	// if the Delete API call was success,
	// then return requeue_event
	// so that in a subsequent reconcile call the absence of the CR is verified by one of the 2 checks above
	return v1alpha1.RECONCILE_AGAIN_ERR
}
