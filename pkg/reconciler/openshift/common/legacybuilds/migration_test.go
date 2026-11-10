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

package legacybuilds

import (
	"context"
	"errors"
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func nodePlugin() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "shared-resource-csi-driver-node", Namespace: Namespace,
			UID: types.UID("legacy-node"), Finalizers: []string{"operator.openshift.io/openshiftbuilds"}},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "legacy-csi"}},
			Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{
				Containers: []corev1.Container{{Name: "driver", Image: "original"}},
				Volumes: []corev1.Volume{{Name: "socket", VolumeSource: corev1.VolumeSource{
					HostPath: &corev1.HostPathVolumeSource{Path: "/var/lib/kubelet/plugins/csi.sharedresource.openshift.io"},
				}}},
			}},
		},
	}
}

func TestStopDaemonSet_DrainsWithoutDeletingData(t *testing.T) {
	ctx := context.Background()
	d := nodePlugin()
	client := kubefake.NewSimpleClientset(d)
	assert.Assert(t, errors.Is(StopDaemonSet(ctx, client, d.Name), v1alpha1.REQUEUE_EVENT_AFTER))
	updated, err := client.AppsV1().DaemonSets(Namespace).Get(ctx, d.Name, metav1.GetOptions{})
	assert.NilError(t, err)
	assert.Equal(t, updated.UID, d.UID)
	assert.DeepEqual(t, updated.Finalizers, d.Finalizers)
	assert.DeepEqual(t, updated.Spec.Template.Spec.Volumes, d.Spec.Template.Spec.Volumes)
	assert.DeepEqual(t, updated.Spec.Template.Spec.Containers, d.Spec.Template.Spec.Containers)
	for _, nodeLabels := range []map[string]string{nil, {"operator.tekton.dev/legacy-builds-csi-retired": "true"}} {
		required := updated.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution
		assert.Equal(t, len(required.NodeSelectorTerms), 1)
		selector := &metav1.LabelSelector{}
		for _, requirement := range required.NodeSelectorTerms[0].MatchExpressions {
			selector.MatchExpressions = append(selector.MatchExpressions, metav1.LabelSelectorRequirement{
				Key: requirement.Key, Operator: metav1.LabelSelectorOperator(requirement.Operator), Values: requirement.Values,
			})
		}
		match, err := metav1.LabelSelectorAsSelector(selector)
		assert.NilError(t, err)
		assert.Assert(t, !match.Matches(labels.Set(nodeLabels)))
	}
	assert.NilError(t, StopDaemonSet(ctx, client, d.Name))
	for _, action := range client.Actions() {
		assert.Assert(t, action.GetVerb() != "delete" && action.GetVerb() != "delete-collection")
	}
}

func TestStopDaemonSet_WaitsForObservedShutdown(t *testing.T) {
	for _, scenario := range []string{"generation", "desired", "current", "misscheduled", "ready", "available", "updated", "running-pod", "terminating-pod"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			d := nodePlugin()
			client := kubefake.NewSimpleClientset(d)
			assert.Assert(t, errors.Is(StopDaemonSet(ctx, client, d.Name), v1alpha1.REQUEUE_EVENT_AFTER))
			d, err := client.AppsV1().DaemonSets(Namespace).Get(ctx, d.Name, metav1.GetOptions{})
			assert.NilError(t, err)
			switch scenario {
			case "generation":
				d.Generation = 2
				d.Status.ObservedGeneration = 1
			case "desired":
				d.Status.DesiredNumberScheduled = 1
			case "current":
				d.Status.CurrentNumberScheduled = 1
			case "misscheduled":
				d.Status.NumberMisscheduled = 1
			case "ready":
				d.Status.NumberReady = 1
			case "available":
				d.Status.NumberAvailable = 1
			case "updated":
				d.Status.UpdatedNumberScheduled = 1
			default:
				pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "old-driver", Namespace: Namespace,
					Labels: map[string]string{"app": "legacy-csi"}}, Status: corev1.PodStatus{Phase: corev1.PodRunning}}
				if scenario == "terminating-pod" {
					now := metav1.Now()
					pod.DeletionTimestamp = &now
				}
				_, err := client.CoreV1().Pods(Namespace).Create(ctx, pod, metav1.CreateOptions{})
				assert.NilError(t, err)
			}
			_, err = client.AppsV1().DaemonSets(Namespace).Update(ctx, d, metav1.UpdateOptions{})
			assert.NilError(t, err)
			assert.Assert(t, errors.Is(StopDaemonSet(ctx, client, d.Name), v1alpha1.REQUEUE_EVENT_AFTER))
		})
	}
}

func TestStopDaemonSet_FailuresRetry(t *testing.T) {
	for _, verb := range []string{"get", "update", "list"} {
		t.Run(verb, func(t *testing.T) {
			ctx := context.Background()
			d := nodePlugin()
			client := kubefake.NewSimpleClientset(d)
			if verb == "list" {
				assert.Assert(t, errors.Is(StopDaemonSet(ctx, client, d.Name), v1alpha1.REQUEUE_EVENT_AFTER))
			}
			want := errors.New("retry")
			client.PrependReactor(verb, "*", func(clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, want
			})
			assert.Assert(t, errors.Is(StopDaemonSet(ctx, client, d.Name), want))
		})
	}
}

func TestStopDaemonSet_AbsentIsNoop(t *testing.T) {
	assert.NilError(t, StopDaemonSet(context.Background(), kubefake.NewSimpleClientset(), "absent"))
}
