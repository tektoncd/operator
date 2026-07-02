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

package namespacesync

import (
	"context"
	"reflect"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	operatorclient "github.com/tektoncd/operator/pkg/client/injection/client"
	tektonConfigInformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonconfig"
	pkgcommon "github.com/tektoncd/operator/pkg/common"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	nsinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/namespace"
	secretinformer "knative.dev/pkg/client/injection/kube/informers/core/v1/secret"
	sainformer "knative.dev/pkg/client/injection/kube/informers/core/v1/serviceaccount"
	rbinformer "knative.dev/pkg/client/injection/kube/informers/rbac/v1/rolebinding"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// NewController initialises the NamespaceSyncController and registers event
// handlers for Namespace, ServiceAccount (pipeline only), Secret, and TektonConfig.
func NewController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)

	nsInf := nsinformer.Get(ctx)
	saInf := sainformer.Get(ctx)
	secretInf := secretinformer.Get(ctx)
	rbInf := rbinformer.Get(ctx)
	tcInf := tektonConfigInformer.Get(ctx)

	rec := &Reconciler{
		kubeClient:         kubeclient.Get(ctx),
		operatorClient:     operatorclient.Get(ctx),
		securityClientSet:  pkgcommon.GetSecurityClient(ctx),
		nsLister:           nsInf.Lister(),
		saLister:           saInf.Lister().ServiceAccounts(""),
		tektonConfigLister: tcInf.Lister(),
	}

	impl := controller.NewContext(ctx, rec, controller.ControllerOptions{
		WorkQueueName: "NamespaceSyncController",
		Logger:        logger,
	})

	// Namespace Add/Update → reconcile that namespace.
	if _, err := nsInf.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			ns, ok := obj.(*corev1.Namespace)
			if !ok || shouldIgnoreNamespace(ns) {
				return
			}
			impl.EnqueueKey(types.NamespacedName{Name: ns.Name})
		},
		UpdateFunc: func(_, newObj interface{}) {
			ns, ok := newObj.(*corev1.Namespace)
			if !ok || shouldIgnoreNamespace(ns) {
				return
			}
			impl.EnqueueKey(types.NamespacedName{Name: ns.Name})
		},
	}); err != nil {
		logger.Panicf("Couldn't register Namespace informer event handler: %v", err)
	}

	// pipeline SA events → reconcile its namespace.
	//
	// Add:    SA was just created (by us or by the existing RBAC reconciler).
	//         Re-enqueue so ensureSecretBindings can bind any secrets that
	//         already existed in the namespace before the SA was present.
	//         This covers design Scenario B (secret arrives before SA).
	//
	// Delete: SA was removed externally — re-enqueue to recreate it.
	//
	// Update: SA contents changed (e.g. admin manually removed a secret
	//         binding) — re-enqueue for self-healing.
	if _, err := saInf.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			sa, ok := obj.(*corev1.ServiceAccount)
			return ok && sa.Name == pipelineSA
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				sa, ok := obj.(*corev1.ServiceAccount)
				if ok {
					impl.EnqueueKey(types.NamespacedName{Name: sa.Namespace})
				}
			},
			UpdateFunc: func(_, newObj interface{}) {
				sa, ok := newObj.(*corev1.ServiceAccount)
				if ok {
					impl.EnqueueKey(types.NamespacedName{Name: sa.Namespace})
				}
			},
			DeleteFunc: func(obj interface{}) {
				sa, ok := obj.(*corev1.ServiceAccount)
				if !ok {
					// Tombstone — extract the SA from the DeletedFinalStateUnknown wrapper.
					if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
						sa, ok = d.Obj.(*corev1.ServiceAccount)
					}
					if !ok {
						return
					}
				}
				impl.EnqueueKey(types.NamespacedName{Name: sa.Namespace})
			},
		},
	}); err != nil {
		logger.Panicf("Couldn't register ServiceAccount informer event handler: %v", err)
	}

	// Secret events → re-enqueue the secret's namespace so that:
	//   - A newly created secret that matches a binding rule is bound immediately.
	//   - A deleted named secret is unbound from the pipeline SA.
	//
	// We use the cluster-wide kube factory here (NOT the namespacedkube one)
	// because we need to observe secrets in all user namespaces, not just the
	// operator's own namespace.
	//
	// To avoid a thundering-herd on clusters that don't use secret bindings,
	// we only enqueue when NamespaceSync has SecretBindings configured AND the
	// secret name/labels match at least one binding rule.
	if _, err := secretInf.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			secret, ok := obj.(*corev1.Secret)
			if !ok {
				return
			}
			if secretMatchesBinding(secret, tcInf.Lister()) {
				impl.EnqueueKey(types.NamespacedName{Name: secret.Namespace})
			}
		},
		DeleteFunc: func(obj interface{}) {
			secret, ok := obj.(*corev1.Secret)
			if !ok {
				if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
					secret, ok = d.Obj.(*corev1.Secret)
				}
				if !ok {
					return
				}
			}
			if secretMatchesBinding(secret, tcInf.Lister()) {
				impl.EnqueueKey(types.NamespacedName{Name: secret.Namespace})
			}
		},
	}); err != nil {
		logger.Panicf("Couldn't register Secret informer event handler: %v", err)
	}

	// RoleBinding Delete → re-enqueue the namespace for self-healing.
	// We only watch the two RoleBindings that NamespaceSyncController owns:
	// pipelines-scc-rolebinding and openshift-pipelines-edit. Watching all
	// RoleBindings would be too noisy; we filter by name at the handler level.
	managedRoleBindings := map[string]struct{}{
		pipelinesSCCRoleBinding: {},
		PipelineRoleBinding:     {},
	}
	if _, err := rbInf.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		DeleteFunc: func(obj interface{}) {
			rb, ok := obj.(*rbacv1.RoleBinding)
			if !ok {
				if d, ok := obj.(cache.DeletedFinalStateUnknown); ok {
					rb, ok = d.Obj.(*rbacv1.RoleBinding)
				}
				if !ok {
					return
				}
			}
			if _, watched := managedRoleBindings[rb.Name]; watched {
				impl.EnqueueKey(types.NamespacedName{Name: rb.Namespace})
			}
		},
	}); err != nil {
		logger.Panicf("Couldn't register RoleBinding informer event handler: %v", err)
	}

	// TektonConfig changed → re-enqueue all namespaces only when the NamespaceSync
	// config itself changed. Unrelated TektonConfig field changes (e.g. pipeline
	// options, pruner settings) must not trigger a full namespace sweep — that
	// would be a thundering-herd problem on large clusters with 1000+ namespaces.
	if _, err := tcInf.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldTC, ok1 := oldObj.(*v1alpha1.TektonConfig)
			newTC, ok2 := newObj.(*v1alpha1.TektonConfig)
			if !ok1 || !ok2 {
				return
			}
			if reflect.DeepEqual(
				oldTC.Spec.Platforms.OpenShift.NamespaceSync,
				newTC.Spec.Platforms.OpenShift.NamespaceSync,
			) {
				return
			}
			for _, name := range allNamespacesFromLister(nsInf.Lister()) {
				impl.EnqueueKey(types.NamespacedName{Name: name})
			}
		},
	}); err != nil {
		logger.Panicf("Couldn't register TektonConfig informer event handler: %v", err)
	}

	return impl
}

// secretMatchesBinding returns true when the given secret matches at least one
// SecretBinding rule in the current TektonConfig. This is used to filter Secret
// events so we only re-enqueue namespaces for secrets we actually care about,
// avoiding unnecessary reconciles for the high volume of dockercfg / token
// secrets that Kubernetes creates automatically.
func secretMatchesBinding(secret *corev1.Secret, lister interface {
	Get(string) (*v1alpha1.TektonConfig, error)
}) bool {
	tc, err := lister.Get(v1alpha1.ConfigResourceName)
	if err != nil {
		return false
	}
	cfg := tc.Spec.Platforms.OpenShift.NamespaceSync
	if cfg == nil || len(cfg.SecretBindings) == 0 {
		return false
	}
	for _, b := range cfg.SecretBindings {
		if b.SecretName != "" && b.SecretName == secret.Name {
			return true
		}
		if b.LabelSelector != nil {
			sel, err := metav1.LabelSelectorAsSelector(b.LabelSelector)
			if err != nil {
				continue
			}
			if sel.Matches(labels.Set(secret.Labels)) {
				return true
			}
		}
	}
	return false
}
