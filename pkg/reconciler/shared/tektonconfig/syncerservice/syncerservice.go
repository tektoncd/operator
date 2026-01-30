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

package syncerservice

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

// IsSyncerServiceEnabled checks if syncer-service should be deployed based on scheduler config
// Syncer-service is enabled when:
// 1. Multi-cluster is NOT disabled (i.e., multi-cluster is enabled)
// 2. The role is Hub
func IsSyncerServiceEnabled(scheduler *v1alpha1.Scheduler) bool {
	if scheduler == nil {
		return false
	}
	return !scheduler.MultiClusterDisabled &&
		scheduler.MultiClusterRole == v1alpha1.MultiClusterRoleHub
}

// EnsureSyncerServiceExists ensures the SyncerService CR exists if conditions are met
func EnsureSyncerServiceExists(ctx context.Context, clients op.SyncerServiceInterface, ss *v1alpha1.SyncerService) (*v1alpha1.SyncerService, error) {
	ssCR, err := GetSyncerService(ctx, clients, v1alpha1.SyncerServiceResourceName)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		if err := CreateSyncerService(ctx, clients, ss); err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	ssCR, err = UpdateSyncerService(ctx, ssCR, ss, clients)
	if err != nil {
		return nil, err
	}

	ready, err := isSyncerServiceReady(ssCR)
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return ssCR, err
}

// EnsureSyncerServiceCRNotExists ensures the SyncerService CR is deleted
func EnsureSyncerServiceCRNotExists(ctx context.Context, clients op.SyncerServiceInterface) error {
	if _, err := GetSyncerService(ctx, clients, v1alpha1.SyncerServiceResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			// SyncerService CR is gone, hence return nil
			return nil
		}
		return err
	}
	// if the Get was successful, try deleting the CR
	if err := clients.Delete(ctx, v1alpha1.SyncerServiceResourceName, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			// SyncerService CR is gone, hence return nil
			return nil
		}
		return fmt.Errorf("SyncerService %q failed to delete: %v", v1alpha1.SyncerServiceResourceName, err)
	}
	// if the Delete API call was success,
	// then return requeue_event
	// so that in a subsequent reconcile call the absence of the CR is verified by one of the 2 checks above
	return v1alpha1.RECONCILE_AGAIN_ERR
}

// GetSyncerService gets the SyncerService CR
func GetSyncerService(ctx context.Context, clients op.SyncerServiceInterface, name string) (*v1alpha1.SyncerService, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

// CreateSyncerService creates the SyncerService CR
func CreateSyncerService(ctx context.Context, clients op.SyncerServiceInterface, ss *v1alpha1.SyncerService) error {
	_, err := clients.Create(ctx, ss, metav1.CreateOptions{})
	return err
}

func isSyncerServiceReady(s *v1alpha1.SyncerService) (bool, error) {
	if s.GetStatus() != nil && s.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(s.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return s.Status.IsReady(), nil
}

// UpdateSyncerService updates the existing SyncerService CR with updated SyncerService CR
func UpdateSyncerService(ctx context.Context, old *v1alpha1.SyncerService, new *v1alpha1.SyncerService, clients op.SyncerServiceInterface) (*v1alpha1.SyncerService, error) {
	updated := false

	// initialize labels(map) object
	if old.ObjectMeta.Labels == nil {
		old.ObjectMeta.Labels = map[string]string{}
	}

	if new.Spec.TargetNamespace != old.Spec.TargetNamespace {
		old.Spec.TargetNamespace = new.Spec.TargetNamespace
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.SyncerServiceOptions, new.Spec.SyncerServiceOptions) {
		old.Spec.SyncerServiceOptions = new.Spec.SyncerServiceOptions
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

	oldLabels, oldHasLabels := old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey]
	newLabels, newHasLabels := new.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey]
	if !oldHasLabels || (newHasLabels && oldLabels != newLabels) {
		old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] = newLabels
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

// GetSyncerServiceCR creates a SyncerService CR from TektonConfig
func GetSyncerServiceCR(config *v1alpha1.TektonConfig, operatorVersion string) *v1alpha1.SyncerService {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
	return &v1alpha1.SyncerService{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.SyncerServiceResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Labels: map[string]string{
				v1alpha1.ReleaseVersionKey: operatorVersion,
			},
		},
		Spec: v1alpha1.SyncerServiceSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			Config: config.Spec.Config,
		},
	}
}
