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
	"log"
	"reflect"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	operatorv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/test/logging"
)

func CreateAddonCR(instance v1alpha1.TektonComponent, client operatorv1alpha1.OperatorV1alpha1Interface) error {
	configInstance := instance.(*v1alpha1.TektonConfig)
	if _, err := ensureTektonAddonExists(client.TektonAddons(), configInstance); err != nil {
		return errors.New(err.Error())
	}
	if _, err := waitForTektonAddonState(client.TektonAddons(), common.AddonResourceName,
		isTektonAddonReady); err != nil {
		log.Println("TektonAddon is not in ready state: ", err)
		return err
	}
	return nil
}

func ensureTektonAddonExists(clients op.TektonAddonInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonAddon, error) {
	taCR, err := GetAddon(clients, common.AddonResourceName)
	if err == nil {
		// if the addon spec is changed then update the instance
		updated := false

		if config.Spec.TargetNamespace != taCR.Spec.TargetNamespace {
			taCR.Spec.TargetNamespace = config.Spec.TargetNamespace
			updated = true
		}

		if !reflect.DeepEqual(config.Spec.Addon.Params, taCR.Spec.Params) {
			taCR.Spec.Params = config.Spec.Addon.Params
			updated = true
		}

		if taCR.ObjectMeta.OwnerReferences == nil {
			ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
			taCR.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ownerRef}
			updated = true
		}

		if updated {
			return clients.Update(context.TODO(), taCR, metav1.UpdateOptions{})
		}

		return taCR, err
	}

	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())

	if apierrs.IsNotFound(err) {
		taCR = &v1alpha1.TektonAddon{
			ObjectMeta: metav1.ObjectMeta{
				Name:            common.AddonResourceName,
				OwnerReferences: []metav1.OwnerReference{ownerRef},
			},
			Spec: v1alpha1.TektonAddonSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: config.Spec.TargetNamespace,
				},
				Params: config.Spec.Addon.Params,
			},
		}
		return clients.Create(context.TODO(), taCR, metav1.CreateOptions{})
	}
	return taCR, err
}

func GetAddon(clients op.TektonAddonInterface, name string) (*v1alpha1.TektonAddon, error) {
	return clients.Get(context.TODO(), name, metav1.GetOptions{})
}

// waitForTektonAddonState polls the status of the TektonAddon called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func waitForTektonAddonState(clients op.TektonAddonInterface, name string,
	inState func(s *v1alpha1.TektonAddon, err error) (bool, error)) (*v1alpha1.TektonAddon, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonAddonState/%s/%s", name, "TektonAddonIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonAddon
	waitErr := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("TektonAddon %s is not in desired state, got: %+v: %w: For more info Please check TektonAddon CR status", name, lastState, waitErr)
	}
	return lastState, nil
}

// isTektonAddonReady will check the status conditions of the TektonAddon and return true if the TektonAddon is ready.
func isTektonAddonReady(s *v1alpha1.TektonAddon, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// TektonAddonCRDelete deletes tha TektonAddon to see if all resources will be deleted
func TektonAddonCRDelete(clients op.TektonAddonInterface, name string) error {
	if _, err := GetAddon(clients, common.AddonResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := clients.Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("TektonAddon %q failed to delete: %v", name, err)
	}
	err := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		_, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("Timed out waiting on TektonAddon to delete %v", err)
	}
	return verifyNoTektonAddonCR(clients)
}

func verifyNoTektonAddonCR(clients op.TektonAddonInterface) error {
	addons, err := clients.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(addons.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonAddon exists")
	}
	return nil
}
