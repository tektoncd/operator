/*
Copyright 2021 The Tekton Authors

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

package pipeline

import (
	"context"
	"fmt"
	"reflect"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"

	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func EnsureTektonPipelineExists(ctx context.Context, clients op.TektonPipelineInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonPipeline, error) {
	tpCR, err := GetPipeline(ctx, clients, v1alpha1.PipelineResourceName)

	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		_, err = CreatePipeline(ctx, clients, config)
		if err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	tpCR, err = UpdatePipeline(ctx, tpCR, config, clients)
	if err != nil {
		return nil, err
	}

	ok, err := isTektonPipelineReady(tpCR, err)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	return tpCR, err
}

func GetPipeline(ctx context.Context, clients op.TektonPipelineInterface, name string) (*v1alpha1.TektonPipeline, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

func CreatePipeline(ctx context.Context, clients op.TektonPipelineInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonPipeline, error) {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())

	tpCR := &v1alpha1.TektonPipeline{
		ObjectMeta: metav1.ObjectMeta{
			Name:            v1alpha1.PipelineResourceName,
			OwnerReferences: []metav1.OwnerReference{ownerRef},
		},
		Spec: v1alpha1.TektonPipelineSpec{
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: config.Spec.TargetNamespace,
			},
			Pipeline: config.Spec.Pipeline,
			Config:   config.Spec.Config,
		},
	}
	_, err := clients.Create(ctx, tpCR, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	return tpCR, err
}

func UpdatePipeline(ctx context.Context, tpCR *v1alpha1.TektonPipeline, config *v1alpha1.TektonConfig, clients op.TektonPipelineInterface) (*v1alpha1.TektonPipeline, error) {
	// if the pipeline spec is changed then update the instance
	updated := false

	if config.Spec.TargetNamespace != tpCR.Spec.TargetNamespace {
		tpCR.Spec.TargetNamespace = config.Spec.TargetNamespace
		updated = true
	}

	if !reflect.DeepEqual(tpCR.Spec.Pipeline, config.Spec.Pipeline) {
		tpCR.Spec.Pipeline = config.Spec.Pipeline
		updated = true
	}

	if !reflect.DeepEqual(tpCR.Spec.Config, config.Spec.Config) {
		tpCR.Spec.Config = config.Spec.Config
		updated = true
	}

	if tpCR.ObjectMeta.OwnerReferences == nil {
		ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
		tpCR.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ownerRef}
		updated = true
	}

	if updated {
		_, err := clients.Update(ctx, tpCR, metav1.UpdateOptions{})
		if err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}
	return tpCR, nil
}

// IsTektonPipelineReady will check the status conditions of the TektonPipeline and return true if the TektonPipeline is ready.
func isTektonPipelineReady(s *v1alpha1.TektonPipeline, err error) (bool, error) {
	upgradePending, errInternal := common.CheckUpgradePending(s)
	if err != nil {
		return false, errInternal
	}
	if upgradePending {
		return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
	}
	return s.Status.IsReady(), err
}

func GetTektonConfig() *v1alpha1.TektonConfig {
	return &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile: "all",
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
		},
	}
}

func EnsureTektonPipelineCRNotExists(ctx context.Context, clients op.TektonPipelineInterface) error {
	if _, err := GetPipeline(ctx, clients, v1alpha1.PipelineResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonPipeline CR is gone, hence return nil
			return nil
		}
		return err
	}
	// if the Get was successful, try deleting the CR
	if err := clients.Delete(ctx, v1alpha1.PipelineResourceName, metav1.DeleteOptions{}); err != nil {
		if apierrs.IsNotFound(err) {
			// TektonPipeline CR is gone, hence return nil
			return nil
		}
		return fmt.Errorf("TektonPipeline %q failed to delete: %v", v1alpha1.PipelineResourceName, err)
	}
	// if the Delete API call was success,
	// then return requeue_event
	// so that in a subsequent reconcile call the absence of the CR is verified by one of the 2 checks above
	return v1alpha1.RECONCILE_AGAIN_ERR
}
