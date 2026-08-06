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

func EnsureTektonKueueExists(clients typedv1alpha1.TektonKueueInterface, names utils.ResourceNames) (*v1alpha1.TektonKueue, error) {
	tk, err := clients.Get(context.TODO(), names.TektonKueue, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		tk := &v1alpha1.TektonKueue{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonKueue,
			},
			Spec: v1alpha1.TektonKueueSpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
				Kueue: v1alpha1.Kueue{
					Disabled: ptr.To(false),
					MultiClusterConfig: v1alpha1.MultiClusterConfig{
						MultiClusterDisabled: true,
					},
				},
			},
		}
		return clients.Create(context.TODO(), tk, metav1.CreateOptions{})
	}
	return tk, err
}

func WaitForTektonKueueState(clients typedv1alpha1.TektonKueueInterface, name string,
	inState func(k *v1alpha1.TektonKueue, err error) (bool, error),
) (*v1alpha1.TektonKueue, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonKueueState/%s/%s", name, "TektonKueueIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonKueue
	waitErr := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektonkueue %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

func IsTektonKueueReady(k *v1alpha1.TektonKueue, err error) (bool, error) {
	return k.Status.IsReady(), err
}

func AssertTektonKueueCRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonKueueState(clients.TektonKueues(), names.TektonKueue, IsTektonKueueReady); err != nil {
		t.Fatalf("TektonKueue CR %q failed to get to the READY status: %v", names.TektonKueue, err)
	}
}
