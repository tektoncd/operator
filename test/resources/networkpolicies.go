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

	"github.com/tektoncd/operator/test/utils"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

// AssertNetworkPoliciesExist polls until all named NetworkPolicies exist in namespace.
func AssertNetworkPoliciesExist(t *testing.T, clients *utils.Clients, namespace string, names []string) {
	t.Helper()
	for _, name := range names {
		np := name
		if err := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
			_, err := clients.KubeClient.NetworkingV1().NetworkPolicies(namespace).Get(ctx, np, metav1.GetOptions{})
			if apierrs.IsNotFound(err) {
				return false, nil
			}
			return err == nil, err
		}); err != nil {
			t.Errorf("NetworkPolicy %q not found in namespace %q: %v", np, namespace, err)
		}
	}
}

// AssertNetworkPoliciesAbsent polls until all named NetworkPolicies are gone from namespace.
func AssertNetworkPoliciesAbsent(t *testing.T, clients *utils.Clients, namespace string, names []string) {
	t.Helper()
	for _, name := range names {
		np := name
		if err := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
			_, err := clients.KubeClient.NetworkingV1().NetworkPolicies(namespace).Get(ctx, np, metav1.GetOptions{})
			if apierrs.IsNotFound(err) {
				return true, nil
			}
			if err != nil {
				return false, err
			}
			return false, nil
		}); err != nil {
			t.Errorf("NetworkPolicy %q still present in namespace %q after timeout: %v", np, namespace, err)
		}
	}
}

// AssertEventListenerReady polls until the EventListener deployment is available.
func AssertEventListenerReady(t *testing.T, clients *utils.Clients, namespace, name string) {
	t.Helper()
	deploymentName := fmt.Sprintf("el-%s", name)
	if err := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		d, err := clients.KubeClient.AppsV1().Deployments(namespace).Get(ctx, deploymentName, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}
		return d.Status.ReadyReplicas >= 1, nil
	}); err != nil {
		t.Fatalf("EventListener deployment %q in namespace %q not ready: %v", deploymentName, namespace, err)
	}
}

// AssertPipelineRunCreated polls until at least one PipelineRun exists in namespace.
func AssertPipelineRunCreated(t *testing.T, clients *utils.Clients, namespace string) {
	t.Helper()
	if err := wait.PollUntilContextTimeout(context.TODO(), utils.Interval, utils.Timeout, true, func(ctx context.Context) (bool, error) {
		prs, err := clients.TektonClient.PipelineRuns(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		return len(prs.Items) > 0, nil
	}); err != nil {
		t.Fatalf("no PipelineRun created in namespace %q: %v", namespace, err)
	}
}

// AssertPipelineRunCountUnchanged waits briefly and then asserts the PipelineRun
// count in namespace has not grown beyond countBefore. Used to verify that a
// non-matching interceptor event did not trigger a PipelineRun.
func AssertPipelineRunCountUnchanged(t *testing.T, clients *utils.Clients, namespace string, countBefore int) {
	t.Helper()
	// Wait a short period to give the system time to process the event if it
	// were going to create a PipelineRun (uses a fraction of the normal timeout).
	shortTimeout := utils.Timeout / 10
	_ = wait.PollUntilContextTimeout(context.TODO(), utils.Interval, shortTimeout, true, func(ctx context.Context) (bool, error) {
		prs, err := clients.TektonClient.PipelineRuns(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		// Keep polling until we see a change (so we fail fast) or time out.
		return len(prs.Items) > countBefore, nil
	})

	prs, err := clients.TektonClient.PipelineRuns(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		t.Fatalf("failed to list PipelineRuns: %v", err)
	}
	if got := len(prs.Items); got > countBefore {
		t.Errorf("expected no new PipelineRun after non-matching event, got %d (was %d)", got, countBefore)
	}
}
