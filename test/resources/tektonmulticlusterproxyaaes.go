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
	"knative.dev/pkg/test/logging"
)

func EnsureTektonMulticlusterProxyAAEExists(clients typedv1alpha1.TektonMulticlusterProxyAAEInterface, names utils.ResourceNames) (*v1alpha1.TektonMulticlusterProxyAAE, error) {
	ks, err := clients.Get(context.TODO(), names.TektonMulticlusterProxyAAE, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		ks := &v1alpha1.TektonMulticlusterProxyAAE{
			ObjectMeta: metav1.ObjectMeta{
				Name: names.TektonMulticlusterProxyAAE,
			},
			Spec: v1alpha1.TektonMulticlusterProxyAAESpec{
				CommonSpec: v1alpha1.CommonSpec{
					TargetNamespace: names.TargetNamespace,
				},
			},
		}
		return clients.Create(context.TODO(), ks, metav1.CreateOptions{})
	}
	return ks, err
}

func WaitForTektonMulticlusterProxyAAEState(clients typedv1alpha1.TektonMulticlusterProxyAAEInterface, name string,
	inState func(s *v1alpha1.TektonMulticlusterProxyAAE, err error) (bool, error),
) (*v1alpha1.TektonMulticlusterProxyAAE, error) {
	span := logging.GetEmitableSpan(context.Background(), fmt.Sprintf("WaitForTektonMulticlusterProxyAAEState/%s/%s", name, "TektonMulticlusterProxyAAEIsReady"))
	defer span.End()

	var lastState *v1alpha1.TektonMulticlusterProxyAAE
	waitErr := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		lastState, err := clients.Get(context.TODO(), name, metav1.GetOptions{})
		return inState(lastState, err)
	})

	if waitErr != nil {
		return lastState, fmt.Errorf("tektonmulticlusterproxyaae %s is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return lastState, nil
}

func IsTektonMulticlusterProxyAAEReady(s *v1alpha1.TektonMulticlusterProxyAAE, err error) (bool, error) {
	return s.Status.IsReady(), err
}

func AssertTektonMulticlusterProxyAAECRReadyStatus(t *testing.T, clients *utils.Clients, names utils.ResourceNames) {
	if _, err := WaitForTektonMulticlusterProxyAAEState(clients.TektonMulticlusterProxyAAEs(), names.TektonMulticlusterProxyAAE, IsTektonMulticlusterProxyAAEReady); err != nil {
		t.Fatalf("TektonMulticlusterProxyAAECR %q failed to get to the READY status: %v", names.TektonMulticlusterProxyAAE, err)
	}
}
