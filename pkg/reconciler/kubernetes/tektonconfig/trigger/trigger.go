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

package trigger

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/test/logging"

	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	operatorv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
)

func CreateTriggerCR(instance v1alpha1.TektonComponent, client operatorv1alpha1.OperatorV1alpha1Interface) error {
	configInstance := instance.(*v1alpha1.TektonConfig)
	if _, err := ensureTektonTriggerExists(client.TektonTriggers(), configInstance.Spec.TargetNamespace); err != nil {
		return errors.New(err.Error())
	}
	if _, err := waitForTektonTriggerState(client.TektonTriggers(), common.TriggerResourceName,
		isTektonTriggerReady); err != nil {
		log.Println("TektonTrigger is not in ready state: ", err)
		return err
	}
	return nil
}

func ensureTektonTriggerExists(clients op.TektonTriggerInterface, targetNS string) (*v1alpha1.TektonTrigger, error) {
	ttCR, err := GetTrigger(clients, common.TriggerResourceName)
	if err == nil {
		return ttCR, err
	}
	if apierrs.IsNotFound(err) {
		ttCR = &v1alpha1.TektonTrigger{
			ObjectMeta: metav1.ObjectMeta{
				Name: common.TriggerResourceName,
			},
			Spec: v1alpha1.TektonTriggerSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: targetNS,
				},
			},
		}
		return clients.Create(context.TODO(), ttCR, metav1.CreateOptions{})
	}
	return ttCR, err
}

func GetTrigger(clients op.TektonTriggerInterface, name string) (*v1alpha1.TektonTrigger, error) {
	return clients.Get(context.TODO(), name, metav1.GetOptions{})
}

// waitForTektonTriggerState polls the status of the TektonTrigger called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func waitForTektonTriggerState(clients op.TektonTriggerInterface, name string,
	inState func(s *v1alpha1.TektonTrigger, err error) (bool, error)) (*v1alpha1.TektonTrigger, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonTriggerState/%s/%s", name, "TektonTriggerIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonTrigger
	waitErr := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektontrigger %s is not in desired state, got: %+v: %w: For more info Please check TektonTrigger CR status", name, lastState, waitErr)
	}
	return lastState, nil
}

// isTektonTriggerReady will check the status conditions of the TektonTrigger and return true if the TektonTrigger is ready.
func isTektonTriggerReady(s *v1alpha1.TektonTrigger, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// TektonTriggerCRDelete deletes tha TektonTrigger to see if all resources will be deleted
func TektonTriggerCRDelete(clients op.TektonTriggerInterface, name string) error {
	if _, err := GetTrigger(clients, common.TriggerResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := clients.Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("TektonTrigger %q failed to delete: %v", name, err)
	}
	err := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		_, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("Timed out waiting on TektonTrigger to delete %v", err)
	}
	return verifyNoTektonTriggerCR(clients)
}

func verifyNoTektonTriggerCR(clients op.TektonTriggerInterface) error {
	triggers, err := clients.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(triggers.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonTrigger exists")
	}
	return nil
}
