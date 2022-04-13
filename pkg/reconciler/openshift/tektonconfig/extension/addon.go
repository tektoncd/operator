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
		return clients.Update(ctx, taCR, metav1.UpdateOptions{})
	}

	return taCR, nil
}

// isTektonAddonReady will check the status conditions of the TektonAddon and return true if the TektonAddon is ready.
func isTektonAddonReady(s *v1alpha1.TektonAddon, err error) (bool, error) {
	upgradePending, errInternal := common.CheckUpgradePending(s)
	if err != nil {
		return false, errInternal
	}
	if upgradePending {
		return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
	}
	return s.Status.IsReady(), err
}

// TektonAddonCRDelete deletes tha TektonAddon to see if all resources will be deleted
func TektonAddonCRDelete(ctx context.Context, clients op.TektonAddonInterface, name string) error {
	if _, err := GetAddon(ctx, clients, v1alpha1.AddonResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := clients.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("TektonAddon %q failed to delete: %v", name, err)
	}
	err := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		_, err := clients.Get(ctx, name, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("Timed out waiting on TektonAddon to delete %v", err)
	}
	return verifyNoTektonAddonCR(ctx, clients)
}

func verifyNoTektonAddonCR(ctx context.Context, clients op.TektonAddonInterface) error {
	addons, err := clients.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(addons.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonAddon exists")
	}
	return nil
}
