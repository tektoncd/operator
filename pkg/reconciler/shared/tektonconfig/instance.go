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
	logger.Debug("ensuring tektonconfig instance")

	waitErr := wait.PollImmediate(RetryInterval, RetryTimeout, func() (bool, error) {
		//note: the code in this block will be retired until
		// an error is returned, or
		// 'true' is returned, or
		// timeout
		instance, err := tc.operatorClientSet.
			OperatorV1alpha1().
			TektonConfigs().Get(ctx, DefaultCRName, metav1.GetOptions{})
		if err == nil {
			return true, nil
		}
		if !apierrs.IsNotFound(err) {
			//log error and retry
			logger.Errorf("error getting Tektonconfig, Name: ", instance.GetName())
			return false, nil
		}
		err = tc.createInstance(ctx)
		if err != nil {
			//log error and retry
			logger.Errorf("error creating Tektonconfig instance, Name: ", instance.GetName())
			return false, nil
		}
		// even if there is no error after create,
		// loop again to ensure the create is successful with a 'get; api call
		return false, nil
	})
	if waitErr != nil {
		// log error and continue
		logger.Error("error ensuring instance of tektonconfig, check retry logs above for more details, %w", waitErr)
		logger.Infof("an instance of TektonConfig need to be created by the user to get Pipelines components installed")
	}
}

func (tc tektonConfig) createInstance(ctx context.Context) error {
	pruneKeep := uint(100)
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
				Resources: []string{"pipelinerun"},
				Keep:      &pruneKeep,
				KeepSince: nil,
				Schedule:  "0 8 * * *",
			},
		},
	}
	tcCR.SetDefaults(ctx)
	_, err := tc.operatorClientSet.OperatorV1alpha1().
		TektonConfigs().Create(ctx, tcCR, metav1.CreateOptions{})
	return err
}
