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
	"context"
	"fmt"
	"time"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	informer "github.com/tektoncd/operator/pkg/client/informers/externalversions/operator/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

var (
	Interval = 10 * time.Second
	Timeout  = 1 * time.Minute
	// DefaultSA is the default service account
	DefaultSA = "pipeline"
)

const (
	PipelineNotReady       = "tekton-pipelines not ready"
	PipelineNotFound       = "tekton-pipelines not installed"
	TriggerNotReady        = "tekton-triggers not ready"
	TriggerNotFound        = "tekton-triggers not installed"
	NamespaceIgnorePattern = "^(openshift|kube)-"
)

func PipelineReady(informer informer.TektonPipelineInformer) (*v1alpha1.TektonPipeline, error) {
	ppln, err := getPipelineRes(informer)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Errorf(PipelineNotFound)
		}
		return nil, err
	}
	upgradePending, err := CheckUpgradePending(ppln)
	if err != nil {
		return nil, err
	}
	if upgradePending {
		return nil, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
	}
	if !ppln.Status.IsReady() {
		return nil, fmt.Errorf(PipelineNotReady)
	}
	return ppln, nil
}

func getPipelineRes(informer informer.TektonPipelineInformer) (*v1alpha1.TektonPipeline, error) {
	res, err := informer.Lister().Get(v1alpha1.PipelineResourceName)
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
	upgradePending, err := CheckUpgradePending(trigger)
	if err != nil {
		return nil, err
	}
	if upgradePending {
		return nil, v1alpha1.DEPENDENCY_UPGRADE_PENDING_ERR
	}
	if !trigger.Status.IsReady() {
		return nil, fmt.Errorf(TriggerNotReady)
	}
	return trigger, nil
}

func getTriggerRes(informer informer.TektonTriggerInformer) (*v1alpha1.TektonTrigger, error) {
	res, err := informer.Lister().Get(v1alpha1.TriggerResourceName)
	return res, err
}

func CheckUpgradePending(tc v1alpha1.TektonComponent) (bool, error) {
	labels := tc.GetLabels()
	ver, ok := labels[v1alpha1.ReleaseVersionKey]
	if !ok {
		return true, nil
	}
	operatorVersion, err := OperatorVersion(context.TODO())
	if err != nil {
		return false, err
	}
	if ver != operatorVersion {
		return true, nil
	}
	return false, nil
}
