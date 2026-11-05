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

package sharedresource

import (
	"testing"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	"github.com/tektoncd/operator/pkg/reconciler/openshift/common/legacybuilds"
	"gotest.tools/v3/assert"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
)

func TestLegacyWorkloadEvents(t *testing.T) {
	for _, name := range []string{legacybuilds.OperatorDeployment, WebhookDeploymentName, legacyNodeDaemonSet} {
		t.Run(name, func(t *testing.T) {
			var object interface{} = &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: legacybuilds.Namespace}}
			if name == legacyNodeDaemonSet {
				object = legacyCSI()
			}
			var keys []types.NamespacedName
			handler := legacyWorkloadEventHandler(func(key types.NamespacedName) { keys = append(keys, key) })
			handler.OnAdd(object, false)
			handler.OnUpdate(object, object)
			handler.OnDelete(cache.DeletedFinalStateUnknown{Key: name, Obj: object})
			assert.Equal(t, len(keys), 3)
			for _, key := range keys {
				assert.Equal(t, key, types.NamespacedName{Name: v1alpha1.SharedResourceResourceName})
			}
		})
	}
}

func TestLegacyWorkloadEventsIgnoreOtherResources(t *testing.T) {
	handler := legacyWorkloadEventHandler(func(types.NamespacedName) { t.Error("unexpected enqueue") })
	for _, object := range []interface{}{
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: WebhookDeploymentName, Namespace: "other"}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: legacyNodeDaemonSet, Namespace: "other"}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: legacybuilds.Namespace}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: legacybuilds.Namespace}},
		&metav1.Status{},
	} {
		handler.OnAdd(object, false)
		handler.OnDelete(cache.DeletedFinalStateUnknown{Key: "ignored", Obj: object})
	}
}
