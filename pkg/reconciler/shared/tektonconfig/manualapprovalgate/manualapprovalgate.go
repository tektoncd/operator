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

package manualapprovalgate

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

func EnsureManualApprovalGateExists(ctx context.Context, clients op.ManualApprovalGateInterface, mag *v1alpha1.ManualApprovalGate) (*v1alpha1.ManualApprovalGate, error) {
	magCR, err := GetManualApprovalGate(ctx, clients, v1alpha1.ManualApprovalGates)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		if err := CreateManualApprovalGate(ctx, clients, mag); err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	magCR, err = UpdateManualApprovalGate(ctx, magCR, mag, clients)
	if err != nil {
		return nil, err
	}

	ready, err := isManualApprovalGateReady(magCR)
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return magCR, err
}

func EnsureManualApprovalGateCRNotExists(ctx context.Context, clients op.ManualApprovalGateInterface) error {
	if _, err := GetManualApprovalGate(ctx, clients, v1alpha1.ManualApprovalGates); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := clients.Delete(ctx, v1alpha1.ManualApprovalGates, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("ManualApprovalGate %q failed to delete: %v", v1alpha1.ManualApprovalGates, err)
	}
	return v1alpha1.RECONCILE_AGAIN_ERR
}

func GetManualApprovalGate(ctx context.Context, clients op.ManualApprovalGateInterface, name string) (*v1alpha1.ManualApprovalGate, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

func CreateManualApprovalGate(ctx context.Context, clients op.ManualApprovalGateInterface, mag *v1alpha1.ManualApprovalGate) error {
	_, err := clients.Create(ctx, mag, metav1.CreateOptions{})
	return err
}

func UpdateManualApprovalGate(ctx context.Context, old *v1alpha1.ManualApprovalGate, new *v1alpha1.ManualApprovalGate, clients op.ManualApprovalGateInterface) (*v1alpha1.ManualApprovalGate, error) {
	updated := false

	if old.ObjectMeta.Labels == nil {
		old.ObjectMeta.Labels = map[string]string{}
	}

	if new.Spec.TargetNamespace != old.Spec.TargetNamespace {
		old.Spec.TargetNamespace = new.Spec.TargetNamespace
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.ManualApproval, new.Spec.ManualApproval) {
		old.Spec.ManualApproval = new.Spec.ManualApproval
		updated = true
	}

	if old.ObjectMeta.OwnerReferences == nil {
		old.ObjectMeta.OwnerReferences = new.ObjectMeta.OwnerReferences
		updated = true
	}

	oldLabels, oldHasLabels := old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey]
	newLabels, newHasLabels := new.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey]
	if !oldHasLabels || (newHasLabels && oldLabels != newLabels) {
		old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] = newLabels
		updated = true
	}

	oldPlatformData := old.ObjectMeta.Annotations[v1alpha1.PlatformDataHashKey]
	newPlatformData := new.ObjectMeta.Annotations[v1alpha1.PlatformDataHashKey]
	if oldPlatformData != newPlatformData {
		if old.ObjectMeta.Annotations == nil {
			old.ObjectMeta.Annotations = map[string]string{}
		}
		old.ObjectMeta.Annotations[v1alpha1.PlatformDataHashKey] = newPlatformData
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

func isManualApprovalGateReady(mag *v1alpha1.ManualApprovalGate) (bool, error) {
	if mag.GetStatus() != nil && mag.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(mag.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return mag.Status.IsReady(), nil
}

func GetManualApprovalGateCR(config *v1alpha1.TektonConfig, operatorVersion string) *v1alpha1.ManualApprovalGate {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
	return &v1alpha1.ManualApprovalGate{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.ManualApprovalGates,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Labels: map[string]string{
				v1alpha1.ReleaseVersionKey: operatorVersion,
			},
		},
		Spec: v1alpha1.ManualApprovalGateSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			ManualApproval: config.Spec.ManualApproval,
		},
	}
}
