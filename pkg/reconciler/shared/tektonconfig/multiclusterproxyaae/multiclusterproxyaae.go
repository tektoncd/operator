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

package multiclusterproxyaae

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

// IsMulticlusterProxyAAEEnabled returns true when TektonMulticlusterProxyAAE should be deployed:
// scheduler is not disabled, multi-cluster is enabled, and role is Hub.
func IsMulticlusterProxyAAEEnabled(tc *v1alpha1.TektonConfig) bool {
	if tc.Spec.Scheduler.IsDisabled() {
		return false
	}
	return !tc.Spec.Scheduler.MultiClusterDisabled &&
		strings.EqualFold(string(tc.Spec.Scheduler.MultiClusterRole), string(v1alpha1.MultiClusterRoleHub))
}

// EnsureTektonMulticlusterProxyAAEComponent ensures TektonMulticlusterProxyAAE CR exists or is removed
// based on TektonConfig scheduler spec (multi-cluster enabled + Hub role).
func EnsureTektonMulticlusterProxyAAEComponent(ctx context.Context, tc *v1alpha1.TektonConfig, clients op.TektonMulticlusterProxyAAEInterface, operatorVersion string) error {
	if !IsMulticlusterProxyAAEEnabled(tc) {
		return EnsureTektonMulticlusterProxyAAECRNotExists(ctx, clients)
	}
	proxy := GetTektonMulticlusterProxyAAECR(tc, operatorVersion)
	_, err := EnsureTektonMulticlusterProxyAAEExists(ctx, clients, proxy)
	return err
}

// GetTektonMulticlusterProxyAAE fetches the TektonMulticlusterProxyAAE CR from the cluster by name.
func GetTektonMulticlusterProxyAAE(ctx context.Context, clients op.TektonMulticlusterProxyAAEInterface, name string) (*v1alpha1.TektonMulticlusterProxyAAE, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

// EnsureTektonMulticlusterProxyAAEExists ensures the TektonMulticlusterProxyAAE CR exists and is ready.
func EnsureTektonMulticlusterProxyAAEExists(ctx context.Context, clients op.TektonMulticlusterProxyAAEInterface, proxy *v1alpha1.TektonMulticlusterProxyAAE) (*v1alpha1.TektonMulticlusterProxyAAE, error) {
	existing, err := GetTektonMulticlusterProxyAAE(ctx, clients, v1alpha1.MultiClusterProxyAAEResourceName)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		if err := CreateTektonMulticlusterProxyAAE(ctx, clients, proxy); err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	existing, err = UpdateTektonMulticlusterProxyAAE(ctx, existing, proxy, clients)
	if err != nil {
		return nil, err
	}

	ready, err := isTektonMulticlusterProxyAAEReady(existing)
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return existing, nil
}

// EnsureTektonMulticlusterProxyAAECRNotExists ensures the TektonMulticlusterProxyAAE CR is deleted.
func EnsureTektonMulticlusterProxyAAECRNotExists(ctx context.Context, clients op.TektonMulticlusterProxyAAEInterface) error {
	if _, err := GetTektonMulticlusterProxyAAE(ctx, clients, v1alpha1.MultiClusterProxyAAEResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := clients.Delete(ctx, v1alpha1.MultiClusterProxyAAEResourceName, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("TektonMulticlusterProxyAAE %q failed to delete: %w", v1alpha1.MultiClusterProxyAAEResourceName, err)
	}
	return v1alpha1.RECONCILE_AGAIN_ERR
}

// GetTektonMulticlusterProxyAAECR returns a TektonMulticlusterProxyAAE CR from TektonConfig.
func GetTektonMulticlusterProxyAAECR(config *v1alpha1.TektonConfig, operatorVersion string) *v1alpha1.TektonMulticlusterProxyAAE {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
	labels := map[string]string{}
	if operatorVersion != "" {
		labels[v1alpha1.ReleaseVersionKey] = operatorVersion
	}
	return &v1alpha1.TektonMulticlusterProxyAAE{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.MultiClusterProxyAAEResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Labels:          labels,
		},
		Spec: v1alpha1.TektonMulticlusterProxyAAESpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
		},
	}
}

// CreateTektonMulticlusterProxyAAE creates the TektonMulticlusterProxyAAE CR.
func CreateTektonMulticlusterProxyAAE(ctx context.Context, clients op.TektonMulticlusterProxyAAEInterface, proxy *v1alpha1.TektonMulticlusterProxyAAE) error {
	_, err := clients.Create(ctx, proxy, metav1.CreateOptions{})
	return err
}

// UpdateTektonMulticlusterProxyAAE updates the existing CR if spec/labels changed.
func UpdateTektonMulticlusterProxyAAE(ctx context.Context, old, new *v1alpha1.TektonMulticlusterProxyAAE, clients op.TektonMulticlusterProxyAAEInterface) (*v1alpha1.TektonMulticlusterProxyAAE, error) {
	updated := false

	if old.ObjectMeta.Labels == nil {
		old.ObjectMeta.Labels = map[string]string{}
	}
	if new.Spec.TargetNamespace != old.Spec.TargetNamespace {
		old.Spec.TargetNamespace = new.Spec.TargetNamespace
		updated = true
	}
	if new.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] != old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] {
		old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] = new.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey]
		updated = true
	}
	if !reflect.DeepEqual(old.Spec.Options, new.Spec.Options) {
		old.Spec.Options = new.Spec.Options
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

func isTektonMulticlusterProxyAAEReady(p *v1alpha1.TektonMulticlusterProxyAAE) (bool, error) {
	if p.GetStatus() != nil && p.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(p.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return p.Status.IsReady(), nil
}
