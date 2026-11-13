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

// Package legacybuilds coordinates retirement of standalone Builds controllers.
// It never deletes user resources, the legacy parent, finalizers, or RBAC.
package legacybuilds

import (
	"context"
	"fmt"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	Namespace          = "openshift-builds"
	OperatorDeployment = "openshift-builds-operator"
)

var CSVGVR = schema.GroupVersionResource{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "clusterserviceversions"}

// Prepare stops the manager and its owning CSV before replacement InstallerSets
// apply shared resources. Recheck even after migration: an OLM upgrade may restart
// the legacy manager with a different CSV.
func Prepare(ctx context.Context, kubeClient kubernetes.Interface, dynClient dynamic.Interface) error {
	if err := pauseCSV(ctx, kubeClient, dynClient); err != nil {
		return err
	}
	return StopDeployment(ctx, kubeClient, OperatorDeployment)
}

func pauseCSV(ctx context.Context, kubeClient kubernetes.Interface, dynClient dynamic.Interface) error {
	d, err := kubeClient.AppsV1().Deployments(Namespace).Get(ctx, OperatorDeployment, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	name := ""
	var ownerUID types.UID
	for _, owner := range d.OwnerReferences {
		if owner.APIVersion == "operators.coreos.com/v1alpha1" && owner.Kind == "ClusterServiceVersion" {
			name, ownerUID = owner.Name, owner.UID
			break
		}
	}
	if name == "" && d.Labels["olm.owner.kind"] == "ClusterServiceVersion" && d.Labels["olm.owner.namespace"] == Namespace {
		name = d.Labels["olm.owner"]
	}
	if name == "" {
		return nil // Non-OLM installation.
	}
	csv, err := dynClient.Resource(CSVGVR).Namespace(Namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if ownerUID != "" && csv.GetUID() != ownerUID {
		return fmt.Errorf("legacy Deployment CSV owner UID %s does not match CSV %s UID %s", ownerUID, name, csv.GetUID())
	}
	deployments, _, err := unstructured.NestedSlice(csv.Object, "spec", "install", "spec", "deployments")
	if err != nil {
		return err
	}
	for _, value := range deployments {
		deployment, ok := value.(map[string]interface{})
		if !ok || deployment["name"] != OperatorDeployment {
			continue
		}
		replicas, found, err := unstructured.NestedInt64(deployment, "spec", "replicas")
		if err != nil {
			return err
		}
		if found && replicas == 0 {
			return nil
		}
		if err := unstructured.SetNestedField(deployment, int64(0), "spec", "replicas"); err != nil {
			return err
		}
		if err := unstructured.SetNestedSlice(csv.Object, deployments, "spec", "install", "spec", "deployments"); err != nil {
			return err
		}
		_, err = dynClient.Resource(CSVGVR).Namespace(Namespace).Update(ctx, csv, metav1.UpdateOptions{})
		return err
	}
	return fmt.Errorf("legacy CSV %s has no %s deployment", name, OperatorDeployment)
}

// StopDeployment waits for observed shutdown, including terminating pods.
func StopDeployment(ctx context.Context, client kubernetes.Interface, name string) error {
	d, err := client.AppsV1().Deployments(Namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if d.Spec.Replicas == nil || *d.Spec.Replicas != 0 {
		zero := int32(0)
		d.Spec.Replicas = &zero
		if _, err := client.AppsV1().Deployments(Namespace).Update(ctx, d, metav1.UpdateOptions{}); err != nil {
			return err
		}
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	if d.Status.ObservedGeneration < d.Generation || d.Status.Replicas != 0 ||
		d.Status.ReadyReplicas != 0 || d.Status.AvailableReplicas != 0 || d.Status.UpdatedReplicas != 0 {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	return waitForPods(ctx, client, name, d.Spec.Selector)
}

// StopDaemonSet drains the old node plugin before another namespace can register
// the same CSI driver/socket. Contradictory affinity matches no node, regardless
// of node labels. Keep the DaemonSet, host paths and finalizers intact.
func StopDaemonSet(ctx context.Context, client kubernetes.Interface, name string) error {
	d, err := client.AppsV1().DaemonSets(Namespace).Get(ctx, name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	const key = "operator.tekton.dev/legacy-builds-csi-retired"
	required := &corev1.NodeSelector{NodeSelectorTerms: []corev1.NodeSelectorTerm{{MatchExpressions: []corev1.NodeSelectorRequirement{
		{Key: key, Operator: corev1.NodeSelectorOpExists},
		{Key: key, Operator: corev1.NodeSelectorOpDoesNotExist},
	}}}}
	affinity := d.Spec.Template.Spec.Affinity
	if affinity == nil || affinity.NodeAffinity == nil ||
		!nodeSelectorRetired(affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution, key) {
		if affinity == nil {
			affinity = &corev1.Affinity{}
			d.Spec.Template.Spec.Affinity = affinity
		}
		if affinity.NodeAffinity == nil {
			affinity.NodeAffinity = &corev1.NodeAffinity{}
		}
		affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = required
		if _, err := client.AppsV1().DaemonSets(Namespace).Update(ctx, d, metav1.UpdateOptions{}); err != nil {
			return err
		}
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	if d.Status.ObservedGeneration < d.Generation || d.Status.DesiredNumberScheduled != 0 ||
		d.Status.CurrentNumberScheduled != 0 || d.Status.NumberMisscheduled != 0 || d.Status.NumberReady != 0 ||
		d.Status.NumberAvailable != 0 || d.Status.UpdatedNumberScheduled != 0 {
		return v1alpha1.REQUEUE_EVENT_AFTER
	}
	return waitForPods(ctx, client, name, d.Spec.Selector)
}

func nodeSelectorRetired(selector *corev1.NodeSelector, key string) bool {
	if selector == nil || len(selector.NodeSelectorTerms) != 1 {
		return false
	}
	var exists, absent bool
	for _, requirement := range selector.NodeSelectorTerms[0].MatchExpressions {
		if requirement.Key == key {
			exists = exists || requirement.Operator == corev1.NodeSelectorOpExists
			absent = absent || requirement.Operator == corev1.NodeSelectorOpDoesNotExist
		}
	}
	return exists && absent
}

func waitForPods(ctx context.Context, client kubernetes.Interface, name string, podSelector *metav1.LabelSelector) error {
	if podSelector == nil || (len(podSelector.MatchLabels) == 0 && len(podSelector.MatchExpressions) == 0) {
		return fmt.Errorf("legacy workload %s has no pod selector", name)
	}
	selector, err := metav1.LabelSelectorAsSelector(podSelector)
	if err != nil {
		return err
	}
	pods, err := client.CoreV1().Pods(Namespace).List(ctx, metav1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return err
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodSucceeded && pod.Status.Phase != corev1.PodFailed {
			return v1alpha1.REQUEUE_EVENT_AFTER
		}
	}
	return nil
}
