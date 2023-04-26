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
	"strings"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"knative.dev/pkg/apis"

	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func EnsureTektonPipelineExists(ctx context.Context, clients op.TektonPipelineInterface, tp *v1alpha1.TektonPipeline) (*v1alpha1.TektonPipeline, error) {
	tpCR, err := GetPipeline(ctx, clients, v1alpha1.PipelineResourceName)

	if err != nil {
		if !apierrs.IsNotFound(err) {
			return nil, err
		}
		if err := CreatePipeline(ctx, clients, tp); err != nil {
			return nil, err
		}
		return nil, v1alpha1.RECONCILE_AGAIN_ERR
	}

	tpCR, err = UpdatePipeline(ctx, tpCR, tp, clients)
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

func GetTektonPipelineCR(config *v1alpha1.TektonConfig) *v1alpha1.TektonPipeline {
	ownerRef := *metav1.NewControllerRef(config, config.GroupVersionKind())
	return &v1alpha1.TektonPipeline{
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
}

func CreatePipeline(ctx context.Context, clients op.TektonPipelineInterface, tp *v1alpha1.TektonPipeline) error {
	_, err := clients.Create(ctx, tp, metav1.CreateOptions{})
	return err
}

func UpdatePipeline(ctx context.Context, old *v1alpha1.TektonPipeline, new *v1alpha1.TektonPipeline, clients op.TektonPipelineInterface) (*v1alpha1.TektonPipeline, error) {
	// if the pipeline spec is changed then update the instance
	updated := false

	if new.Spec.TargetNamespace != old.Spec.TargetNamespace {
		old.Spec.TargetNamespace = new.Spec.TargetNamespace
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.Pipeline, new.Spec.Pipeline) {
		old.Spec.Pipeline = new.Spec.Pipeline
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.Config, new.Spec.Config) {
		old.Spec.Config = new.Spec.Config
		updated = true
	}

	if !reflect.DeepEqual(old.Spec.Performance, new.Spec.Performance) {
		old.Spec.Performance = new.Spec.Performance
		updated = true
	}

	if old.ObjectMeta.OwnerReferences == nil {
		old.ObjectMeta.OwnerReferences = new.ObjectMeta.OwnerReferences
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

// IsTektonPipelineReady will check the status conditions of the TektonPipeline and return true if the TektonPipeline is ready.
func isTektonPipelineReady(s *v1alpha1.TektonPipeline, err error) (bool, error) {
	if s.GetStatus() != nil && s.GetStatus().GetCondition(apis.ConditionReady) != nil {
		if strings.Contains(s.GetStatus().GetCondition(apis.ConditionReady).Message, v1alpha1.UpgradePending) {
			return false, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
		}
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
