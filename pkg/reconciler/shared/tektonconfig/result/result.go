/*
Copyright 2024 The Tekton Authors

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

package result

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

// This Ensure TektonResult CR is exist or not
// if it exist then update it otherwise creates a new TektonResult CR
func EnsureTektonResultExists(ctx context.Context, clients op.TektonResultInterface, tr *v1alpha1.TektonResult) (*v1alpha1.TektonResult, error) {
	trCR, err := GetResult(ctx, clients, v1alpha1.ResultResourceName)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		if err := CreateResult(ctx, clients, tr); err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	trCR, err = UpdateResult(ctx, trCR, tr, clients)
	if err != nil {
		return nil, err
	}

	ready, err := isTektonResultReady(trCR)
	if err != nil {
		return nil, err
	}
	if !ready {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return trCR, err
}

// This Ensure TektonResult CR is deleted successfully
func EnsureTektonResultCRNotExists(ctx context.Context, clients op.TektonResultInterface) error {
	if _, err := GetResult(ctx, clients, v1alpha1.ResultResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonResult CR is gone, hence return nil
			return nil
		}
		return err
	}
	// if the Get was successful, try deleting the CR
	if err := clients.Delete(ctx, v1alpha1.ResultResourceName, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonResult CR is gone, hence return nil
			return nil
		}
		return fmt.Errorf("TektonResult %q failed to delete: %v", v1alpha1.ResultResourceName, err)
	}
	// if the Delete API call was success,
	// then return requeue_event
	// so that in a subsequent reconcile call the absence of the CR is verified by one of the 2 checks above
	return v1alpha1.RECONCILE_AGAIN_ERR
}

// Get the TektonResult CR
func GetResult(ctx context.Context, clients op.TektonResultInterface, name string) (*v1alpha1.TektonResult, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

// Create the TektonResult CR
func CreateResult(ctx context.Context, clients op.TektonResultInterface, tr *v1alpha1.TektonResult) error {
	_, err := clients.Create(ctx, tr, metav1.CreateOptions{})
	return err
}

func isTektonResultReady(s *v1alpha1.TektonResult) (bool, error) {
	if s.GetStatus() != nil && s.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(s.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return s.Status.IsReady(), nil
}

// This update the existing TektonResult CR with updated TektonResult CR
func UpdateResult(ctx context.Context, old *v1alpha1.TektonResult, new *v1alpha1.TektonResult, clients op.TektonResultInterface) (*v1alpha1.TektonResult, error) {
	// if the result spec is changed then update the instance
	updated := false

	// initialize labels(map) object
	if old.ObjectMeta.Labels == nil {
		old.ObjectMeta.Labels = map[string]string{}
	}

	if new.Spec.TargetNamespace != old.Spec.TargetNamespace {
		old.Spec.TargetNamespace = new.Spec.TargetNamespace
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.ResultsAPIProperties, new.Spec.ResultsAPIProperties) {
		old.Spec.ResultsAPIProperties = new.Spec.ResultsAPIProperties
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.LokiStackProperties, new.Spec.LokiStackProperties) {
		old.Spec.LokiStackProperties = new.Spec.LokiStackProperties
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.ResultsAPIProperties.Options, new.Spec.ResultsAPIProperties.Options) {
		old.Spec.ResultsAPIProperties.Options = new.Spec.ResultsAPIProperties.Options
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

// GetTektonResultCR create a TektonResult CR
func GetTektonResultCR(config *v1alpha1.TektonConfig, operatorVersion string) *v1alpha1.TektonResult {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
	return &v1alpha1.TektonResult{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.ResultResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Labels: map[string]string{
				v1alpha1.ReleaseVersionKey: operatorVersion,
			},
		},
		Spec: v1alpha1.TektonResultSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			ResultsAPIProperties: config.Spec.Result.ResultsAPIProperties,
			LokiStackProperties:  config.Spec.Result.LokiStackProperties,
		},
	}
}
