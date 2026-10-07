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

// Package sharedresource reconciles the singleton SharedResource CR from the
// TektonConfig spec.platforms.openshift.sharedResource field. TektonConfig owns
// the CR; the standalone SharedResource controller reconciles it and installs
// the component. SharedResource is OpenShift-only.
package sharedresource

import (
	"context"
	"reflect"
	"strings"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

// IsEnabled reports whether the SharedResource component is requested by the
// TektonConfig spec. SharedResource is OpenShift-only.
func IsEnabled(config *v1alpha1.TektonConfig) bool {
	if !v1alpha1.IsOpenShiftPlatform() {
		return false
	}
	src := config.Spec.Platforms.OpenShift.SharedResource
	return src != nil && src.Enabled
}

// Get fetches the singleton SharedResource CR
func Get(ctx context.Context, client operatorclient.SharedResourceInterface) (*v1alpha1.SharedResource, error) {
	return client.Get(ctx, v1alpha1.SharedResourceResourceName, metav1.GetOptions{})
}

// CreateOrUpdate creates or updates the SharedResource CR to match the desired spec carried by TektonConfig.
func CreateOrUpdate(ctx context.Context, client operatorclient.SharedResourceInterface, config *v1alpha1.TektonConfig, operatorVersion string) (*v1alpha1.SharedResource, error) {
	sr, err := client.Get(ctx, v1alpha1.SharedResourceResourceName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		if sr, err = client.Create(ctx, create(config, operatorVersion), metav1.CreateOptions{}); err != nil {
			return nil, err
		}
		return sr, v1alpha1.RECONCILE_AGAIN_ERR
	}

	if sr, updated := update(sr, create(config, operatorVersion)); updated {
		if sr, err := client.Update(ctx, sr, metav1.UpdateOptions{}); err != nil {
			return nil, err
		} else {
			return sr, v1alpha1.RECONCILE_AGAIN_ERR
		}
	}

	if ready, err := isReady(sr); err != nil {
		return nil, err
	} else if !ready {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return sr, nil
}

// DeleteIfExists deletes the singleton SharedResource CR if it exists.
func DeleteIfExists(ctx context.Context, client operatorclient.SharedResourceInterface) error {
	if err := client.Delete(ctx, v1alpha1.SharedResourceResourceName, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return v1alpha1.RECONCILE_AGAIN_ERR
}

func create(config *v1alpha1.TektonConfig, operatorVersion string) *v1alpha1.SharedResource {
	src := config.Spec.Platforms.OpenShift.SharedResource
	ownerReference := *metav1.NewControllerRef(config, config.GroupVersionKind())

	sr := &v1alpha1.SharedResource{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.SharedResourceResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
			Labels: map[string]string{
				v1alpha1.ReleaseVersionKey: operatorVersion,
			},
		},
		Spec: v1alpha1.SharedResourceSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			SharedResourceConfig: *src,
			Config:               config.Spec.Config,
			NetworkPolicy:        config.Spec.NetworkPolicy,
		},
	}
	return sr
}

func update(old *v1alpha1.SharedResource, new *v1alpha1.SharedResource) (*v1alpha1.SharedResource, bool) {
	updated := false

	if old.ObjectMeta.Labels == nil {
		old.ObjectMeta.Labels = map[string]string{}
	}

	if new.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] != old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] {
		old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] = new.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey]
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.SharedResourceConfig, new.Spec.SharedResourceConfig) {
		old.Spec.SharedResourceConfig = new.Spec.SharedResourceConfig
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.Config, new.Spec.Config) {
		old.Spec.Config = new.Spec.Config
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.NetworkPolicy, new.Spec.NetworkPolicy) {
		old.Spec.NetworkPolicy = new.Spec.NetworkPolicy
		updated = true
	}

	if old.ObjectMeta.OwnerReferences == nil {
		old.ObjectMeta.OwnerReferences = new.ObjectMeta.OwnerReferences
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
	return old, updated
}

func isReady(sr *v1alpha1.SharedResource) (bool, error) {
	if sr.GetStatus() != nil && sr.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(sr.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return sr.Status.IsReady(), nil
}
