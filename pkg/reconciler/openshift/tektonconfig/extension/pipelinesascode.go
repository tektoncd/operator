/*
Copyright 2022 The Tekton Authors

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

func EnsureOpenShiftPipelinesAsCodeExists(ctx context.Context, clients op.OpenShiftPipelinesAsCodeInterface, config *v1alpha1.TektonConfig, operatorVersion string) (*v1alpha1.OpenShiftPipelinesAsCode, error) {
	opacCR, err := GetPAC(ctx, clients, v1alpha1.OpenShiftPipelinesAsCodeName)
	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		if _, err = createOPAC(ctx, clients, config, operatorVersion); err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	opacCR, err = updateOPAC(ctx, opacCR, config, clients, operatorVersion)
	if err != nil {
		return nil, err
	}

	ok, err := isOPACReady(opacCR, err)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return opacCR, err
}

func createOPAC(ctx context.Context, clients op.OpenShiftPipelinesAsCodeInterface, config *v1alpha1.TektonConfig, operatorVersion string) (*v1alpha1.OpenShiftPipelinesAsCode, error) {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())

	opacCR := &v1alpha1.OpenShiftPipelinesAsCode{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.OpenShiftPipelinesAsCodeName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
			Labels: map[string]string{
				v1alpha1.ReleaseVersionKey: operatorVersion,
			},
		},
		Spec: v1alpha1.OpenShiftPipelinesAsCodeSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			Config: config.Spec.Config,
			PACSettings: v1alpha1.PACSettings{
				Settings:                 config.Spec.Platforms.OpenShift.PipelinesAsCode.Settings,
				AdditionalPACControllers: config.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.AdditionalPACControllers,
			},
		},
	}
	if _, err := clients.Create(ctx, opacCR, metav1.CreateOptions{}); err != nil {
		return nil, err
	}
	return opacCR, nil
}

func GetPAC(ctx context.Context, clients op.OpenShiftPipelinesAsCodeInterface, name string) (*v1alpha1.OpenShiftPipelinesAsCode, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

func updateOPAC(ctx context.Context, opacCR *v1alpha1.OpenShiftPipelinesAsCode, config *v1alpha1.TektonConfig,
	clients op.OpenShiftPipelinesAsCodeInterface, operatorVersion string,
) (*v1alpha1.OpenShiftPipelinesAsCode, error) {
	// if the pac spec is changed then update the instance
	updated := false

	// initialize labels(map) object
	if opacCR.ObjectMeta.Labels == nil {
		opacCR.ObjectMeta.Labels = map[string]string{}
	}

	if config.Spec.TargetNamespace != opacCR.Spec.TargetNamespace {
		opacCR.Spec.TargetNamespace = config.Spec.TargetNamespace
		updated = true
	}

	if !reflect.DeepEqual(opacCR.Spec.Config, config.Spec.Config) {
		opacCR.Spec.Config = config.Spec.Config
		updated = true
	}

	if !reflect.DeepEqual(opacCR.Spec.PACSettings.Settings, config.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.Settings) {
		opacCR.Spec.PACSettings.Settings = config.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.Settings
		updated = true
	}

	if !reflect.DeepEqual(opacCR.Spec.PACSettings.Options, config.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.Options) {
		opacCR.Spec.PACSettings.Options = config.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.Options
		updated = true
	}

	if !reflect.DeepEqual(opacCR.Spec.PACSettings.AdditionalPACControllers, config.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.AdditionalPACControllers) {
		opacCR.Spec.PACSettings.AdditionalPACControllers = config.Spec.Platforms.OpenShift.PipelinesAsCode.PACSettings.AdditionalPACControllers
		updated = true
	}

	if opacCR.ObjectMeta.OwnerReferences == nil {
		ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
		opacCR.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ownerRef}
		updated = true
	}

	oldLabels, oldHasLabels := opacCR.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey]
	if !oldHasLabels || (oldLabels != operatorVersion) {
		opacCR.ObjectMeta.Labels[v1alpha1.ReleaseVersionKey] = operatorVersion
		updated = true
	}

	if updated {
		_, err := clients.Update(ctx, opacCR, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return opacCR, nil
}

// isOPACReady will check the status conditions of the OpenShiftPipelinesAsCode and return true if the OpenShiftPipelinesAsCode is ready.
func isOPACReady(s *v1alpha1.OpenShiftPipelinesAsCode, err error) (bool, error) {
	if s.GetStatus() != nil && s.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(s.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
	}
	return s.Status.IsReady(), err
}

func EnsureOpenShiftPipelinesAsCodeCRNotExists(ctx context.Context, clients op.OpenShiftPipelinesAsCodeInterface) error {
	if _, err := GetPAC(ctx, clients, v1alpha1.OpenShiftPipelinesAsCodeName); err != nil {
		if apierrs.IsNotFound(err) {
			// OpenShiftPipelinesAsCode CR is gone, hence return nil
			return nil
		}
		return err
	}
	// if the Get was successful, try deleting the CR
	if err := clients.Delete(ctx, v1alpha1.OpenShiftPipelinesAsCodeName, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			// OpenShiftPipelinesAsCode CR is gone, hence return nil
			return nil
		}
		return fmt.Errorf("OpenShiftPipelinesAsCode %q failed to delete: %v", v1alpha1.OpenShiftPipelinesAsCodeName, err)
	}
	// if the Delete API call was success,
	// then return requeue_event
	// so that in a subsequent reconcile call the absence of the CR is verified by one of the 2 checks above
	return v1alpha1.RECONCILE_AGAIN_ERR
}
