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

package resources

import (
	"context"
	"fmt"

	"knative.dev/pkg/test/logging"

	"github.com/tektoncd/operator/test/utils"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	pipelinev1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EnsureTektonPipelineExists creates a TektonPipeline with the name names.TektonPipeline, if it does not exist.
func EnsureTektonPipelineExists(clients pipelinev1alpha1.TektonPipelineInterface, names utils.ResourceNames) (*v1alpha1.TektonPipeline, error) {
	// If this function is called by the upgrade tests, we only create the custom resource, if it does not exist.
	ks, err := clients.Get(context.TODO(), names.TektonPipeline, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ks := &v1alpha1.TektonPipeline{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonPipeline,
			},
			Spec: v1alpha1.TektonPipelineSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: "tekton-pipelines",
				},
			},
		}
		return clients.Create(context.TODO(), ks, metav1.CreateOptions{})
	}
	return ks, err
}

// WaitForTektonPipelineState polls the status of the TektonPipeline called name
// from client every `interval` until `inState` returns `true` indicating it
// is done, returns an error or timeout.
func WaitForTektonPipelineState(clients pipelinev1alpha1.TektonPipelineInterface, name string,
	inState func(s *v1alpha1.TektonPipeline, err error) (bool, error)) (*v1alpha1.TektonPipeline, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonPipelineState/%s/%s", name, "TektonPipelineIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonPipeline
	waitErr := wait.PollImmediate(Interval, Timeout, func() (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektonpipeline %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

// IsTektonPipelineReady will check the status conditions of the TektonPipeline and return true if the TektonPipeline is ready.
func IsTektonPipelineReady(s *v1alpha1.TektonPipeline, err error) (bool, error) {
	return s.Status.IsReady(), err
}
