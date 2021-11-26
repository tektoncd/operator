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

func CreateDashboardCR(ctx context.Context, instance v1alpha1.TektonComponent, client operatorv1alpha1.OperatorV1alpha1Interface) error {
	configInstance := instance.(*v1alpha1.TektonConfig)
	if _, err := ensureTektonDashboardExists(ctx, client.TektonDashboards(), configInstance); err != nil {
		return errors.New(err.Error())
	}
	if _, err := waitForTektonDashboardState(ctx, client.TektonDashboards(), v1alpha1.DashboardResourceName,
		isTektonDashboardReady); err != nil {
		log.Println("TektonDashboard is not in ready state: ", err)
		return err
	}
	return nil
}

func ensureTektonDashboardExists(ctx context.Context, clients op.TektonDashboardInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonDashboard, error) {
	tdCR, err := GetDashboard(ctx, clients, v1alpha1.DashboardResourceName)
	if err == nil {
		// if the dashboard spec is changed then update the instance
		updated := false

		if config.Spec.TargetNamespace != tdCR.Spec.TargetNamespace {
			tdCR.Spec.TargetNamespace = config.Spec.TargetNamespace
			updated = true
		}

		if !reflect.DeepEqual(tdCR.Spec.DashboardProperties, config.Spec.Dashboard.DashboardProperties) {
			tdCR.Spec.DashboardProperties = config.Spec.Dashboard.DashboardProperties
			updated = true
		}

		if !reflect.DeepEqual(tdCR.Spec.Config, config.Spec.Config) {
			tdCR.Spec.Config = config.Spec.Config
			updated = true
		}

		if tdCR.ObjectMeta.OwnerReferences == nil {
			ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
			tdCR.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ownerRef}
			updated = true
		}

		if updated {
			return clients.Update(ctx, tdCR, metav1.UpdateOptions{})
		}

		return tdCR, err
	}

	if apierrs.IsNotFound(err) {
		tdCR = &v1alpha1.TektonDashboard{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1alpha1.DashboardResourceName,
			},
			Spec: v1alpha1.TektonDashboardSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: config.Spec.TargetNamespace,
				},
				Config:              config.Spec.Config,
				DashboardProperties: config.Spec.Dashboard.DashboardProperties,
			},
		}
		return clients.Create(ctx, tdCR, metav1.CreateOptions{})
	}
	return tdCR, err
}

func GetDashboard(ctx context.Context, clients op.TektonDashboardInterface, name string) (*v1alpha1.TektonDashboard, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

// waitForTektonDashboardState polls the status of the TektonDashboard called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func waitForTektonDashboardState(ctx context.Context, clients op.TektonDashboardInterface, name string,
	inState func(s *v1alpha1.TektonDashboard, err error) (bool, error)) (*v1alpha1.TektonDashboard, error) {
	span := logging.GetEmitableSpan(ctx, fmt.Sprintf("WaitForTektonDashboardState/%s/%s", name, "TektonDashboardIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonDashboard
	waitErr := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		lastState, err := clients.Get(ctx, name, metav1.GetOptions{})
		return inState(lastState, err)
	})
	if waitErr != nil {
		return lastState, fmt.Errorf("TektonDashboard %s is not in desired state, got: %+v: %w: For more info Please check TektonDashboard CR status", name, lastState, waitErr)
	}
	return lastState, nil
}

// isTektonDashboardReady will check the status conditions of the TektonDashboard and return true if the TektonDashboard is ready.
func isTektonDashboardReady(s *v1alpha1.TektonDashboard, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// TektonDashboardCRDelete deletes tha TektonDashboard to see if all resources will be deleted
func TektonDashboardCRDelete(ctx context.Context, clients op.TektonDashboardInterface, name string) error {
	if _, err := GetDashboard(ctx, clients, v1alpha1.DashboardResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := clients.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("TektonDashboard %q failed to delete: %v", name, err)
	}
	err := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		_, err := clients.Get(ctx, name, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("Timed out waiting on TektonDashboard to delete %v", err)
	}
	return verifyNoTektonDashboardCR(ctx, clients)
}

func verifyNoTektonDashboardCR(ctx context.Context, clients op.TektonDashboardInterface) error {
	dashboards, err := clients.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(dashboards.Items) > 0 {
		return errors.New("Unable to verify cluster-scoped resources are deleted if any TektonDashboard exists")
	}
	return nil
}
