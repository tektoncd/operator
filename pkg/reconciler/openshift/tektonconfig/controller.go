/*
Copyright 2020 The Tekton Authors

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

	"github.com/tektoncd/operator/pkg/reconciler/common"
	k8s_ctrl "github.com/tektoncd/operator/pkg/reconciler/kubernetes/tektonconfig"
	"k8s.io/apimachinery/pkg/types"
	namespaceinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/namespace"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"
)

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	ctrl := k8s_ctrl.NewExtendedController(OpenShiftExtension)(ctx, cmw)
	namespaceInformer := namespaceinformer.Get(ctx)
	namespaceInformer.Informer().AddEventHandler(controller.HandleAll(enqueueCustomName(ctrl, common.ConfigResourceName)))
	return ctrl
}

// enqueueCustomName adds an event with name `config` in work queue so that
// whenever a namespace event occurs, the TektonConfig reconciler get triggered.
// This is required because we want to get our TektonConfig reconciler triggered
// for already existing and new namespaces, without manual intervention like adding
// a label/annotation on namespace to make it manageable by Tekton controller.
// This will also filter the namespaces by regex `^(openshift|kube)-`
// and enqueue only when namespace doesn't match the regex
func enqueueCustomName(impl *controller.Impl, name string) func(obj interface{}) {
	return func(obj interface{}) {
		object, err := kmeta.DeletionHandlingAccessor(obj)
		if err == nil && !nsRegex.MatchString(object.GetName()) {
			impl.EnqueueKey(types.NamespacedName{Namespace: "", Name: name})
		}
	}
}
