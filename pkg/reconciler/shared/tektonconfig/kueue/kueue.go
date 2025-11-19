/*
Copyright 2025 The Tekton Authors

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

package kueue

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	clientset "github.com/tektoncd/operator/pkg/client/clientset/versioned"
	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func EnsureTektonKueueExists(ctx context.Context, clients op.TektonKueueInterface, tk *v1alpha1.TektonKueue) (*v1alpha1.TektonKueue, error) {
	tpCR, err := GetKueue(ctx, clients, v1alpha1.TektonKueueResourceName)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		if err := CreateKueue(ctx, clients, tk); err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	tpCR, err = UpdateKueue(ctx, tpCR, tk, clients)
	if err != nil {
		return nil, err
	}

	ok, err := isTektonKueueReady(tpCR, err)
	if err != nil {
		return nil, err
	}

	if !ok {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return tpCR, err
}

func GetKueue(ctx context.Context, clients op.TektonKueueInterface, name string) (*v1alpha1.TektonKueue, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

func GetTektonKueueCR(config *v1alpha1.TektonConfig, operatorVersion string) *v1alpha1.TektonKueue {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
	return &v1alpha1.TektonKueue{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.TektonKueueResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Labels: map[string]string{
				v1alpha1.ReleaseVersionKey: operatorVersion,
			},
		},
		Spec: v1alpha1.TektonKueueSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			Config: config.Spec.Config,
			Kueue:  config.Spec.Kueue,
		},
	}
}

func CreateKueue(ctx context.Context, clients op.TektonKueueInterface, kueue *v1alpha1.TektonKueue) error {
	_, err := clients.Create(ctx, kueue, metav1.CreateOptions{})
	return err
}

func UpdateKueue(ctx context.Context, old *v1alpha1.TektonKueue, new *v1alpha1.TektonKueue, clients op.TektonKueueInterface) (*v1alpha1.TektonKueue, error) {
	// if the kueue spec is changed then update the instance
	updated := false
	// initialize labels(map) object
	if old.ObjectMeta.Labels == nil {
		old.ObjectMeta.Labels = map[string]string{}
	}

	if new.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] != old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] {
		old.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] = new.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey]
		updated = true
	}

	if new.Spec.TargetNamespace != old.Spec.TargetNamespace {
		old.Spec.TargetNamespace = new.Spec.TargetNamespace
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.Kueue, new.Spec.Kueue) {
		old.Spec.Kueue = new.Spec.Kueue
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.Config, new.Spec.Config) {
		old.Spec.Config = new.Spec.Config
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

// isTektonKueueReady will check the status conditions of the TektonKueue and return true if the TektonKueue is ready.
func isTektonKueueReady(s *v1alpha1.TektonKueue, err error) (bool, error) {
	if s.GetStatus() != nil && s.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(s.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return s.Status.IsReady(), err
}

func EnsureTektonKueueCRNotExists(ctx context.Context, clients op.TektonKueueInterface) error {
	if _, err := GetKueue(ctx, clients, v1alpha1.TektonKueueResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonKueue CR is gone, hence return nil
			return nil
		}
		return err
	}
	// if the Get was successful, try deleting the CR
	if err := clients.Delete(ctx, v1alpha1.TektonKueueResourceName, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonKueue CR is gone, hence return nil
			return nil
		}
		return fmt.Errorf("TektonKueue %q failed to delete: %v", v1alpha1.TektonKueueResourceName, err)
	}
	// if the Delete API call was success,
	// then return requeue_event
	// so that in a subsequent reconcile call the absence of the CR is verified by one of the 2 checks above
	return v1alpha1.RECONCILE_AGAIN_ERR
}

func EnsureComponent(ctx context.Context, tc *v1alpha1.TektonConfig, operatorClientSet clientset.Interface, operatorVersion string) error {
	tektonKueue := GetTektonKueueCR(tc, operatorVersion)
	if _, err := EnsureTektonKueueExists(ctx, operatorClientSet.OperatorV1alpha1().TektonKueues(), tektonKueue); err != nil {
		tc.Status.MarkComponentNotReady(fmt.Sprintf("TektonKueue %s", err.Error()))
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	return nil
}
