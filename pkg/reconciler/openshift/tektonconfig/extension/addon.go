/*
Copyright 2020 The Tekton Authors

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

package extension

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

func EnsureTektonAddonExists(ctx context.Context, clients op.TektonAddonInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonAddon, error) {
	taCR, err := GetAddon(ctx, clients, v1alpha1.AddonResourceName)

	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		if _, err = createAddon(ctx, clients, config); err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	taCR, err = updateAddon(ctx, taCR, config, clients)
	if err != nil {
		return nil, err
	}

	ok, err := isTektonAddonReady(taCR, err)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return taCR, err
}

func createAddon(ctx context.Context, clients op.TektonAddonInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonAddon, error) {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())

	taCR := &v1alpha1.TektonAddon{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.AddonResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonAddonSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			Addon: v1alpha1.Addon{
				Params: config.Spec.Addon.Params,
			},
			Config: config.Spec.Config,
		},
	}
	if _, err := clients.Create(ctx, taCR, metav1.CreateOptions{}); err != nil {
		return nil, err
	}
	return taCR, nil
}

func GetAddon(ctx context.Context, clients op.TektonAddonInterface, name string) (*v1alpha1.TektonAddon, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

func updateAddon(ctx context.Context, taCR *v1alpha1.TektonAddon, config *v1alpha1.TektonConfig,
	clients op.TektonAddonInterface) (*v1alpha1.TektonAddon, error) {
	// if the addon spec is changed then update the instance
	updated := false

	if config.Spec.TargetNamespace != taCR.Spec.TargetNamespace {
		taCR.Spec.TargetNamespace = config.Spec.TargetNamespace
		updated = true
	}

	if !reflect.DeepEqual(config.Spec.Addon, taCR.Spec.Addon) {
		taCR.Spec.Addon = config.Spec.Addon
		updated = true
	}

	if !reflect.DeepEqual(taCR.Spec.Config, config.Spec.Config) {
		taCR.Spec.Config = config.Spec.Config
		updated = true
	}

	if taCR.ObjectMeta.OwnerReferences == nil {
		ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
		taCR.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ownerRef}
		updated = true
	}

	if updated {
		_, err := clients.Update(ctx, taCR, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return taCR, nil
}

// isTektonAddonReady will check the status conditions of the TektonAddon and return true if the TektonAddon is ready.
func isTektonAddonReady(s *v1alpha1.TektonAddon, err error) (bool, error) {
	if s.GetStatus() != nil && s.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(s.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return s.Status.IsReady(), nil
}

func EnsureTektonAddonCRNotExists(ctx context.Context, clients op.TektonAddonInterface) error {
	if _, err := GetAddon(ctx, clients, v1alpha1.AddonResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonAddon CR is gone, hence return nil
			return nil
		}
		return err
	}
	// if the Get was successful, try deleting the CR
	if err := clients.Delete(ctx, v1alpha1.AddonResourceName, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonAddon CR is gone, hence return nil
			return nil
		}
		return fmt.Errorf("TektonAddon %q failed to delete: %v", v1alpha1.AddonResourceName, err)
	}
	// if the Delete API call was success,
	// then return requeue_event
	// so that in a subsequent reconcile call the absence of the CR is verified by one of the 2 checks above
	return v1alpha1.RECONCILE_AGAIN_ERR
}
