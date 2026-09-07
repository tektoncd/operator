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

// Package shipwrightbuild reconciles the singleton ShipwrightBuild CR from the
// TektonConfig spec.platforms.openshift.builds field. TektonConfig owns the CR;
// the standalone ShipwrightBuild controller reconciles it and installs the
// components.
package shipwrightbuild

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

// targetNamespace is the namespace where the OpenShift Builds components are
// installed. It matches the namespace used by the upstream openshift-builds
// operator.
//const targetNamespace = "openshift-builds"

func IsEnabled(config *v1alpha1.TektonConfig) bool {
	if v1alpha1.IsOpenShiftPlatform() {
		return *config.Spec.Platforms.OpenShift.ShipwrightBuild.Enabled
	}
	return *config.Spec.Platforms.Kubernetes.ShipwrightBuild.Enabled
}

// Get fetches the singleton ShipwrightBuild CR
func Get(ctx context.Context, client operatorclient.ShipwrightBuildInterface) (*v1alpha1.ShipwrightBuild, error) {
	return client.Get(ctx, v1alpha1.ShipwrightBuildResourceName, metav1.GetOptions{})
}

// CreateOrUpdate creates or updates ShipwrightBuild CR to match the desired spec carried by TektonConfig.
func CreateOrUpdate(ctx context.Context, client operatorclient.ShipwrightBuildInterface, config *v1alpha1.TektonConfig, operatorVersion string) (*v1alpha1.ShipwrightBuild, error) {
	sb, err := client.Get(ctx, v1alpha1.ShipwrightBuildResourceName, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return nil, err
		}
		if sb, err = client.Create(ctx, create(config, operatorVersion), metav1.CreateOptions{}); err != nil {
			return nil, err
		}
		return sb, v1alpha1.RECONCILE_AGAIN_ERR
	}

	if sb, updated := update(sb, create(config, operatorVersion)); updated {
		if sb, err := client.Update(ctx, sb, metav1.UpdateOptions{}); err != nil {
			return nil, err
		} else {
			return sb, v1alpha1.RECONCILE_AGAIN_ERR
		}
	}

	if ready, err := isReady(sb); err != nil {
		return nil, err
	} else if !ready {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return sb, nil
}

// DeleteIfExists deletes the singleton ShipwrightBuild CR if it exists.
func DeleteIfExists(ctx context.Context, client operatorclient.ShipwrightBuildInterface) error {
	if err := client.Delete(ctx, v1alpha1.ShipwrightBuildResourceName, metav1.DeleteOptions{}); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return err
	}
	return v1alpha1.RECONCILE_AGAIN_ERR
}

func create(config *v1alpha1.TektonConfig, operatorVersion string) *v1alpha1.ShipwrightBuild {
	var sbc *v1alpha1.ShipwrightBuildConfig

	if v1alpha1.IsOpenShiftPlatform() {
		sbc = config.Spec.Platforms.OpenShift.ShipwrightBuild
	} else {
		sbc = config.Spec.Platforms.Kubernetes.ShipwrightBuild
	}
	ownerReference := *metav1.NewControllerRef(config, config.GroupVersionKind())

	sb := &v1alpha1.ShipwrightBuild{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.ShipwrightBuildResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerReference},
			Labels: map[string]string{
				v1alpha1.ReleaseVersionKey: operatorVersion,
			},
		},
		Spec: v1alpha1.ShipwrightBuildSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			ShipwrightBuildConfig: *sbc,
			Config:                config.Spec.Config,
			NetworkPolicy:         config.Spec.NetworkPolicy,
		},
	}
	return sb
}

func update(old *v1alpha1.ShipwrightBuild, new *v1alpha1.ShipwrightBuild) (*v1alpha1.ShipwrightBuild, bool) {
	updated := false

	if old.ObjectMeta.Labels == nil {
		old.ObjectMeta.Labels = map[string]string{}
	}

	if new.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] != old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] {
		old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] = new.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey]
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.ShipwrightBuildConfig, new.Spec.ShipwrightBuildConfig) {
		old.Spec.ShipwrightBuildConfig = new.Spec.ShipwrightBuildConfig
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
	return old, updated
}

func isReady(sb *v1alpha1.ShipwrightBuild) (bool, error) {
	if sb.GetStatus() != nil && sb.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(sb.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return sb.Status.IsReady(), nil
}
