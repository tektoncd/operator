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

func EnsureTektonDashboardExists(ctx context.Context, clients op.TektonDashboardInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonDashboard, error) {
	tdCR, err := GetDashboard(ctx, clients, v1alpha1.DashboardResourceName)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		if _, err = createDashboard(ctx, clients, config); err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	tdCR, err = updateDashboard(ctx, tdCR, config, clients)
	if err != nil {
		return nil, err
	}

	ok, err := isTektonDashboardReady(tdCR, err)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return tdCR, err
}

func GetDashboard(ctx context.Context, clients op.TektonDashboardInterface, name string) (*v1alpha1.TektonDashboard, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

func createDashboard(ctx context.Context, clients op.TektonDashboardInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonDashboard, error) {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())

	tdCR := &v1alpha1.TektonDashboard{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.DashboardResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
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

func updateDashboard(ctx context.Context, tdCR *v1alpha1.TektonDashboard, config *v1alpha1.TektonConfig,
	clients op.TektonDashboardInterface) (*v1alpha1.TektonDashboard, error) {
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
		_, err := clients.Update(ctx, tdCR, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return tdCR, nil
}

// isTektonDashboardReady will check the status conditions of the TektonDashboard and return true if the TektonDashboard is ready.
func isTektonDashboardReady(s *v1alpha1.TektonDashboard, err error) (bool, error) {
	if s.GetStatus() != nil && s.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(s.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return s.Status.IsReady(), err
}

// EnsureTektonDashboardCRNotExists deletes the singleton instance of TektonDashboard
// and ensures the instance is removed checking whether in exists in a subsequent invocation
func EnsureTektonDashboardCRNotExists(ctx context.Context, clients op.TektonDashboardInterface) error {
	if _, err := GetDashboard(ctx, clients, v1alpha1.DashboardResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonDashBoard CR is gone, hence return nil
			return nil
		}
		return err
	}
	// if the Get was successful, try deleting the CR
	if err := clients.Delete(ctx, v1alpha1.DashboardResourceName, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonDashBoard CR is gone, hence return nil
			return nil
		}
		return fmt.Errorf("TektonDashboard %q failed to delete: %v", v1alpha1.DashboardResourceName, err)
	}
	// if the Delete API call was success,
	// then return requeue_event
	// so that in a subsequent reconcile call the absence of the CR is verified by one of the 2 checks above
	return v1alpha1.RECONCILE_AGAIN_ERR
}
