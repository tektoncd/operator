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

package common

import (
	"fmt"
	"time"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	informer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	PipelineResourceName  = "pipeline"
	TriggerResourceName   = "trigger"
	DashboardResourceName = "dashboard"
	AddonResourceName     = "addon"
	ConfigResourceName    = "config"
	ProfileLite           = "lite"
	ProfileBasic          = "basic"
	ProfileAll            = "all"
	Interval              = 10 * time.Second
	Timeout               = 1 * time.Minute
)

const (
	PipelineNotReady = "tekton-pipelines not ready"
	PipelineNotFound = "tekton-pipelines not installed"
	TriggerNotReady  = "tekton-triggers not ready"
	TriggerNotFound  = "tekton-triggers not installed"
)

func PipelineReady(informer informer.TektonPipelineInformer) (*v1alpha1.TektonPipeline, error) {
	ppln, err := getPipelineRes(informer)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf(PipelineNotFound)
		}
		return nil, err
	}

	if len(ppln.Status.Conditions) != 0 {
		if ppln.Status.Conditions[0].Status != corev1.ConditionTrue {
			return nil, fmt.Errorf(PipelineNotReady)
		}
	}
	return ppln, nil
}

func getPipelineRes(informer informer.TektonPipelineInformer) (*v1alpha1.TektonPipeline, error) {
	res, err := informer.Lister().Get(PipelineResourceName)
	return res, err
}

func TriggerReady(informer informer.TektonTriggerInformer) (*v1alpha1.TektonTrigger, error) {
	trigger, err := getTriggerRes(informer)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf(TriggerNotFound)
		}
		return nil, err
	}

	if len(trigger.Status.Conditions) != 0 {
		if trigger.Status.Conditions[0].Status != corev1.ConditionTrue {
			return nil, fmt.Errorf(TriggerNotReady)
		}
	}
	return trigger, nil
}

func getTriggerRes(informer informer.TektonTriggerInformer) (*v1alpha1.TektonTrigger, error) {
	res, err := informer.Lister().Get(TriggerResourceName)
	return res, err
}
