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

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	pipelineinformer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var WatchedResourceName = "pipeline"

const (
	PipelineNotReady = "tekton-pipelines not ready"
	PipelineNotFound = "tekton-pipelines not installed"
)

func PipelineReady(informer pipelineinformer.TektonPipelineInformer) (*v1alpha1.TektonPipeline, error) {
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

func getPipelineRes(informer pipelineinformer.TektonPipelineInformer) (*v1alpha1.TektonPipeline, error) {
	res, err := informer.Lister().Get(WatchedResourceName)
	return res, err
}
