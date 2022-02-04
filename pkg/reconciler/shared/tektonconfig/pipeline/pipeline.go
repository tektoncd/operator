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

func CreatePipelineCR(ctx context.Context, instance v1alpha1.TektonComponent, client operatorv1alpha1.OperatorV1alpha1Interface) error {
	configInstance := instance.(*v1alpha1.TektonConfig)
	if _, err := ensureTektonPipelineExists(ctx, client.TektonPipelines(), configInstance); err != nil {
		return errors.New(err.Error())
	}
	if _, err := waitForTektonPipelineState(ctx, client.TektonPipelines(), v1alpha1.PipelineResourceName,
		isTektonPipelineReady); err != nil {
		log.Println("TektonPipeline is not in ready state: ", err)
		return err
	}
	return nil
}

func ensureTektonPipelineExists(ctx context.Context, clients op.TektonPipelineInterface, config *v1alpha1.TektonConfig) (*v1alpha1.TektonPipeline, error) {
	tpCR, err := GetPipeline(ctx, clients, v1alpha1.PipelineResourceName)
	if err == nil {
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
			return clients.Update(ctx, tpCR, metav1.UpdateOptions{})
		}

		return tpCR, err
	}

	if apierrs.IsNotFound(err) {
		tpCR = &v1alpha1.TektonPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: v1alpha1.PipelineResourceName,
			},
			Spec: v1alpha1.TektonPipelineSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: config.Spec.TargetNamespace,
				},
				Pipeline: config.Spec.Pipeline,
				Config:   config.Spec.Config,
			},
		}

		return clients.Create(ctx, tpCR, metav1.CreateOptions{})
	}
	return tpCR, err
}

func GetPipeline(ctx context.Context, clients op.TektonPipelineInterface, name string) (*v1alpha1.TektonPipeline, error) {
	return clients.Get(ctx, name, metav1.GetOptions{})
}

// WaitForTektonPipelineState polls the status of the TektonPipeline called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func waitForTektonPipelineState(ctx context.Context, clients op.TektonPipelineInterface, name string,
	inState func(s *v1alpha1.TektonPipeline, err error) (bool, error)) (*v1alpha1.TektonPipeline, error) {
	span := logging.GetEmitableSpan(ctx, fmt.Sprintf("WaitForTektonPipelineState/%s/%s", name, "TektonPipelineIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonPipeline
	waitErr := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		lastState, err := clients.Get(ctx, name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektonpipeline %s is not in desired state, got: %+v: %w: For more info Please check TektonPipeline CR status", name, lastState, waitErr)
	}
	return lastState, nil
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

// TektonPipelineCRDelete deletes tha TektonPipeline to see if all resources will be deleted
func TektonPipelineCRDelete(ctx context.Context, clients op.TektonPipelineInterface, name string) error {
	if _, err := GetPipeline(ctx, clients, v1alpha1.PipelineResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := clients.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("TektonPipeline %q failed to delete: %v", name, err)
	}
	err := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		_, err := clients.Get(ctx, name, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("Timed out waiting on TektonPipeline to delete %v", err)
	}
	return verifyNoTektonPipelineCR(ctx, clients)
}

func verifyNoTektonPipelineCR(ctx context.Context, clients op.TektonPipelineInterface) error {
	pipelines, err := clients.List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}
	if len(pipelines.Items) > 0 {
		return errors.New("TektonPipeline still exists")
	}
	return nil
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
