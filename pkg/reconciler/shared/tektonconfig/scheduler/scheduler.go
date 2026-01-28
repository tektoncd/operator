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

package scheduler

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

const (
	KUEUE_GVK = "kueue.x-k8s.io/v1beta1"
	CERT_GVK  = "cert-manager.io/v1"
)

func EnsureTektonSchedulerExists(ctx context.Context, clients op.TektonSchedulerInterface, newScheduler *v1alpha1.TektonScheduler) (*v1alpha1.TektonScheduler, error) {

	// Update MultiKueueOverride
	// If MultiCluster is enabled and MultiClusterRole=Hub then MultiKueueOverride should be true
	newScheduler.Spec.Config.MultiKueueOverride = !newScheduler.Spec.MultiClusterDisabled && newScheduler.Spec.MultiClusterRole == v1alpha1.MultiClusterRoleHub
	TektonScheduler, err := GetTektonScheduler(ctx, clients, v1alpha1.TektonSchedulerResourceName)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		if err := CreateScheduler(ctx, clients, newScheduler); err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	TektonScheduler, err = UpdateScheduler(ctx, TektonScheduler, newScheduler, clients)
	if err != nil {
		return nil, err
	}

	ok, err := isTektonSchedulerReady(TektonScheduler, err)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return TektonScheduler, err
}

func GetTektonScheduler(ctx context.Context, clients op.TektonSchedulerInterface, name string) (*v1alpha1.TektonScheduler, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

func GetTektonSchedulerCR(config *v1alpha1.TektonConfig, operatorVersion string) *v1alpha1.TektonScheduler {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
	return &v1alpha1.TektonScheduler{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.TektonSchedulerResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Labels: map[string]string{
				v1alpha1.ReleaseVersionKey: operatorVersion,
			},
		},
		Spec: v1alpha1.TektonSchedulerSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			Scheduler: config.Spec.Scheduler,
		},
	}
}

func CreateScheduler(ctx context.Context, clients op.TektonSchedulerInterface, scheduler *v1alpha1.TektonScheduler) error {
	_, err := clients.Create(ctx, scheduler, metav1.CreateOptions{})
	return err
}

func UpdateScheduler(ctx context.Context, old *v1alpha1.TektonScheduler, new *v1alpha1.TektonScheduler, clients op.TektonSchedulerInterface) (*v1alpha1.TektonScheduler, error) {
	// if the scheduler spec is changed then update the instance
	updated := false
	// initialize labels(map) object
	if old.ObjectMeta.Labels == nil {
		old.ObjectMeta.Labels = map[string]string{}
	}

	if new.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] != old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] {
		old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] = new.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey]
		updated = true
	}

	if new.Spec.TargetNamespace != old.Spec.TargetNamespace {
		old.Spec.TargetNamespace = new.Spec.TargetNamespace
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.Scheduler, new.Spec.Scheduler) {
		old.Spec.Scheduler = new.Spec.Scheduler
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.SchedulerConfig, new.Spec.SchedulerConfig) {
		old.Spec.SchedulerConfig = new.Spec.SchedulerConfig
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.MultiClusterConfig, new.Spec.MultiClusterConfig) {
		old.Spec.MultiClusterConfig = new.Spec.MultiClusterConfig
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

func isTektonSchedulerReady(s *v1alpha1.TektonScheduler, err error) (bool, error) {
	if s.GetStatus() != nil && s.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(s.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return s.Status.IsReady(), err
}

func EnsureTektonSchedulerCRNotExists(ctx context.Context, clients op.TektonSchedulerInterface) error {
	if _, err := GetTektonScheduler(ctx, clients, v1alpha1.TektonSchedulerResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonScheduler CR is gone, hence return nil
			return nil
		}
		return err
	}
	// if the Get was successful, try deleting the CR
	if err := clients.Delete(ctx, v1alpha1.TektonSchedulerResourceName, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonScheduler CR is gone, hence return nil
			return nil
		}
		return fmt.Errorf("TektonScheduler %q failed to delete: %v", v1alpha1.TektonSchedulerResourceName, err)
	}
	// if the Delete API call was success,
	// then return requeue_event
	// so that in a subsequent reconcile call the absence of the CR is verified by one of the 2 checks above
	return v1alpha1.RECONCILE_AGAIN_ERR
}

// EnsureTektonComponent validates that specific component is  deployed on cluster
func EnsureTektonComponent(ctx context.Context, tc *v1alpha1.TektonConfig, operatorClientSet clientset.Interface, operatorVersion string) error {
	if tc.Spec.Scheduler.IsDisabled() {
		// If TektonScheduler is disabled then uninstall the components
		return EnsureTektonSchedulerCRNotExists(ctx, operatorClientSet.OperatorV1alpha1().TektonSchedulers())
	}
	// Cert-Manager should also be pre-installed
	_, err := operatorClientSet.Discovery().ServerResourcesForGroupVersion(CERT_GVK)
	if err != nil {
		tc.Status.MarkComponentNotReady(fmt.Sprintf("Please install cert-manager (%s) First, %s ", CERT_GVK, err.Error()))
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	// Before Installing Scheduler, Make sure that Upstream Kueue is installed
	_, err = operatorClientSet.Discovery().ServerResourcesForGroupVersion(KUEUE_GVK)
	if err != nil {
		tc.Status.MarkComponentNotReady(fmt.Sprintf("Please install kueue (%s) First, %s ", KUEUE_GVK, err.Error()))
		return v1alpha1.REQUEUE_EVENT_AFTER
	}

	// If Scheduler is installed then create TektonScheduler CR
	TektonScheduler := GetTektonSchedulerCR(tc, operatorVersion)
	if _, err := EnsureTektonSchedulerExists(ctx, operatorClientSet.OperatorV1alpha1().TektonSchedulers(), TektonScheduler); err != nil {
		tc.Status.MarkComponentNotReady(fmt.Sprintf("TektonScheduler : %s", err.Error()))
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	return nil
}
