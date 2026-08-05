/*
Copyright 2026 The Tekton Authors

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
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	typedv1alpha1 "github.com/tektoncd/operator/pkg/client/clientset/versioned/typed/operator/v1alpha1"
	"github.com/tektoncd/operator/test/utils"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/utils/ptr"
	"knative.dev/pkg/test/logging"
)

func EnsureTektonSchedulerExists(clients typedv1alpha1.TektonSchedulerInterface, names utils.ResourceNames) (*v1alpha1.TektonScheduler, error) {
	ks, err := clients.Get(context.TODO(), names.TektonScheduler, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ks := &v1alpha1.TektonScheduler{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonScheduler,
			},
			Spec: v1alpha1.TektonSchedulerSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
				Scheduler: v1alpha1.Scheduler{
					Disabled: ptr.To(false),
					MultiClusterConfig: v1alpha1.MultiClusterConfig{
						MultiClusterDisabled: true,
					},
				},
			},
		}
		return clients.Create(context.TODO(), ks, metav1.CreateOptions{})
	}
	return ks, err
}

func WaitForTektonSchedulerState(clients typedv1alpha1.TektonSchedulerInterface, name string,
	inState func(s *v1alpha1.TektonScheduler, err error) (bool, error),
) (*v1alpha1.TektonScheduler, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonSchedulerState/%s/%s", name, "TektonSchedulerIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonScheduler
	waitErr := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektonscheduler %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

func IsTektonSchedulerReady(s *v1alpha1.TektonScheduler, err error) (bool, error) {
	return s.Status.IsReady(), err
}

func AssertTektonSchedulerCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonSchedulerState(clients.TektonSchedulers(), names.TektonScheduler, IsTektonSchedulerReady); err != nil {
		t.Fatalf("TektonSchedulerCR %q failed to get to the READY status: %v", names.TektonScheduler, err)
	}
}
