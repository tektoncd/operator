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

package pipeline

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"

	op "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	operatorv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/common"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"knative.dev/pkg/test/logging"
)

func CreatePipelineCR(instance v1alpha1.TektonComponent, client operatorv1alpha1.OperatorV1alpha1Interface) error {
	configInstance := instance.(*v1alpha1.TektonConfig)
	if _, err := ensureTektonPipelineExists(client.TektonPipelines(), configInstance.Spec.TargetNamespace, configInstance); err != nil {
		return errors.New(err.Error())
	}
	if _, err := waitForTektonPipelineState(client.TektonPipelines(), common.PipelineResourceName,
		isTektonPipelineReady); err != nil {
		log.Println("TektonPipeline is not in ready state: ", err)
		return err
	}
	return nil
}

func ensureTektonPipelineExists(clients op.TektonPipelineInterface, targetNS string, instance *v1alpha1.TektonConfig) (*v1alpha1.TektonPipeline, error) {
	tpCR, err := GetPipeline(clients, common.PipelineResourceName)
	if err == nil {
		return tpCR, err
	}

	ownerRef := v1.OwnerReference{
		APIVersion: "operator.tekton.dev/v1alpha1",
		Kind:       "TektonPipeline",
		Name:       instance.Name,
		UID:        instance.ObjectMeta.UID,
	}
	if apierrs.IsNotFound(err) {
		tpCR = &v1alpha1.TektonPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name:            common.PipelineResourceName,
				OwnerReferences: []v1.OwnerReference{ownerRef},
			},
			Spec: v1alpha1.TektonPipelineSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: targetNS,
				},
			},
		}
		return clients.Create(context.TODO(), tpCR, metav1.CreateOptions{})
	}
	return tpCR, err
}

func GetPipeline(clients op.TektonPipelineInterface, name string) (*v1alpha1.TektonPipeline, error) {
	return clients.Get(context.TODO(), name, metav1.GetOptions{})
}

// WaitForTektonPipelineState polls the status of the TektonPipeline called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func waitForTektonPipelineState(clients op.TektonPipelineInterface, name string,
	inState func(s *v1alpha1.TektonPipeline, err error) (bool, error)) (*v1alpha1.TektonPipeline, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonPipelineState/%s/%s", name, "TektonPipelineIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonPipeline
	waitErr := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektonpipeline %s is not in desired state, got: %+v: %w: For more info Please check TektonPipeline CR status", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsTektonPipelineReady will check the status conditions of the TektonPipeline and return true if the TektonPipeline is ready.
func isTektonPipelineReady(s *v1alpha1.TektonPipeline, err error) (bool, error) {
	return s.Status.IsReady(), err
}

// TektonPipelineCRDelete deletes tha TektonPipeline to see if all resources will be deleted
func TektonPipelineCRDelete(clients op.TektonPipelineInterface, name string) error {
	if _, err := GetPipeline(clients, common.PipelineResourceName); err != nil {
		if apierrs.IsNotFound(err) {
			return nil
		}
		return err
	}
	if err := clients.Delete(context.TODO(), name, metav1.DeleteOptions{}); err != nil {
		return fmt.Errorf("TektonPipeline %q failed to delete: %v", name, err)
	}
	err := wait.PollImmediate(common.Interval, common.Timeout, func() (bool, error) {
		_, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return true, nil
		}
		return false, err
	})
	if err != nil {
		return fmt.Errorf("Timed out waiting on TektonPipeline to delete %v", err)
	}
	return verifyNoTektonPipelineCR(clients)
}

func verifyNoTektonPipelineCR(clients op.TektonPipelineInterface) error {
	pipelines, err := clients.List(context.TODO(), metav1.ListOptions{})
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
			Name: common.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile: "all",
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: "tekton-pipelines",
			},
		},
	}
}
