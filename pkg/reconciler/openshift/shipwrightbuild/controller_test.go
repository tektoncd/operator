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

package shipwrightbuild

import (
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/kubernetes/shipwrightbuild"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

func TestLegacyDeploymentEvents(t *testing.T) {
	for _, name := range []string{legacyOperatorDeployment, shipwrightbuild.ControllerDeploymentName, shipwrightbuild.WebhookDeploymentName} {
		t.Run(name, func(t *testing.T) {
			var keys []types.NamespacedName
			handler := legacyDeploymentEventHandler(func(key types.NamespacedName) {
				keys = append(keys, key)
			})
			deployment := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: legacyNamespace}}
			handler.OnAdd(deployment, true)
			handler.OnUpdate(deployment, deployment.DeepCopy())
			handler.OnDelete(deployment)
			handler.OnDelete(cache.DeletedFinalStateUnknown{Key: legacyNamespace + "/" + name, Obj: deployment})
			assert.Equal(t, len(keys), 4)
			for _, key := range keys {
				assert.Equal(t, key, types.NamespacedName{Name: v1alpha1.ShipwrightBuildResourceName})
			}
		})
	}
}

func TestLegacyDeploymentEventsIgnoreUnrelatedResources(t *testing.T) {
	handler := legacyDeploymentEventHandler(func(types.NamespacedName) {
		t.Fatal("unrelated resource triggered a legacy handoff reconcile")
	})
	for _, obj := range []interface{}{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: legacyOperatorDeployment, Namespace: "other"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", Namespace: legacyNamespace}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "shared-resource-csi-driver-webhook", Namespace: legacyNamespace}},
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: legacyOperatorDeployment, Namespace: legacyNamespace}},
		cache.DeletedFinalStateUnknown{Obj: "invalid"},
		nil,
	} {
		handler.OnAdd(obj, false)
		handler.OnUpdate(obj, obj)
		handler.OnDelete(obj)
	}
}
