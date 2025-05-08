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

package tektonconfig

import (
	"context"
	"os"
	"time"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/client/clientset/versioned"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
)

const (
	// RetryInterval specifies the time between two polls.
	RetryInterval = 10 * time.Second

	// RetryTimeout specifies the timeout for the function PollImmediate to
	// reach a certain status.
	RetryTimeout = 5 * time.Minute

	// DefaultCRName specifies the default targetnamespaceto be used
	// in autocreated TektonConfig instance
	DefaultCRName = "config"
)

type tektonConfig struct {
	operatorClientSet versioned.Interface
	kubeClientSet     kubernetes.Interface
	namespace         string
}

func newTektonConfig(operatorClientSet versioned.Interface, kubeClientSet kubernetes.Interface) tektonConfig {

	return tektonConfig{
		operatorClientSet: operatorClientSet,
		kubeClientSet:     kubeClientSet,
		namespace:         os.Getenv("DEFAULT_TARGET_NAMESPACE"),
	}
}

// try to ensure an instance of TektonConfig exists
// if there is an error log error,and continue (an instance of TektonConfig will
// then need to be created by the user to get Tekton Pipelines components installed
func (tc tektonConfig) ensureInstance(ctx context.Context) {
	logger := logging.FromContext(ctx)
	logger.Debugw("Ensuring TektonConfig instance exists")

	waitErr := wait.PollUntilContextTimeout(ctx, RetryInterval, RetryTimeout, true, func(ctx context.Context) (bool, error) {
		//note: the code in this block will be retired until
		// an error is returned, or
		// 'true' is returned, or
		// timeout
		instance, err := tc.operatorClientSet.
			OperatorV1alpha1().
			TektonConfigs().Get(ctx, DefaultCRName, metav1.GetOptions{})
		if err == nil {
			logger.Infow("Found existing TektonConfig instance",
				"name", instance.GetName(),
				"generation", instance.GetGeneration(),
				"resourceVersion", instance.GetResourceVersion())
			return true, nil
		}
		if !apierrs.IsNotFound(err) {
			logger.Errorw("Error getting TektonConfig", "error", err)
			return false, nil
		}

		logger.Debugw("TektonConfig instance not found, creating new instance")
		err = tc.createInstance(ctx)
		if err != nil {
			logger.Errorw("Failed to create TektonConfig instance", "error", err)
			return false, nil
		}

		logger.Infow("TektonConfig instance created, verifying on next iteration")
		// even if there is no error after create,
		// loop again to ensure the create is successful with a 'get; api call
		return false, nil
	})

	if waitErr != nil {
		logger.Errorw("Failed to ensure TektonConfig instance exists after timeout",
			"retryInterval", RetryInterval.String(),
			"timeout", RetryTimeout.String(),
			"error", waitErr)
		logger.Warnw("TektonConfig instance must be created manually to install Pipelines components")
	} else {
		logger.Infow("Successfully ensured TektonConfig instance exists")
	}
}

func (tc tektonConfig) createInstance(ctx context.Context) error {
	pruneKeep := v1alpha1.PrunerDefaultKeep
	tcCR := &v1alpha1.TektonConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: v1alpha1.ConfigResourceName,
		},
		Spec: v1alpha1.TektonConfigSpec{
			Profile: v1alpha1.ProfileAll,
			CommonSpec: v1alpha1.CommonSpec{
				TargetNamespace: tc.namespace,
			},
			Pruner: v1alpha1.Prune{
				Disabled:  false,
				Resources: v1alpha1.PruningDefaultResources,
				Keep:      &pruneKeep,
				KeepSince: nil,
				Schedule:  v1alpha1.PrunerDefaultSchedule,
			},
			// Disable the TektonPruner by default
			TektonPruner: v1alpha1.Pruner{
				Disabled: true,
			},
		},
	}
	tcCR.SetDefaults(ctx)
	_, err := tc.operatorClientSet.OperatorV1alpha1().
		TektonConfigs().Create(ctx, tcCR, metav1.CreateOptions{})
	return err
}
