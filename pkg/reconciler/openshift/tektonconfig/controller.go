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
	"os"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	openshiftconfigclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	openshiftpipelinesascodeinformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/openshiftpipelinesascode"
	tektonAddoninformer "github.com/tektoncd/operator/pkg/client/injection/informers/operator/v1alpha1/tektonaddon"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"github.com/tektoncd/operator/pkg/reconciler/shared/tektonconfig"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
)

// NewController initializes the controller and is called by the generated code
// Registers eventhandlers to enqueue events
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)
	ctrl := tektonconfig.NewExtensibleController(OpenShiftExtension)(ctx, cmw)
	if _, err := tektonAddoninformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1alpha1.TektonConfig{}),
		Handler:    controller.HandleAll(ctrl.EnqueueControllerOf),
	}); err != nil {
		logger.Panicf("Couldn't register TektonAddon informer event handler: %w", err)
	}
	if _, err := openshiftpipelinesascodeinformer.Get(ctx).Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterController(&v1alpha1.TektonConfig{}),
		Handler:    controller.HandleAll(ctrl.EnqueueControllerOf),
	}); err != nil {
		logger.Panicf("Couldn't register OpenShiftPipelinesAsCode informer event handler: %w", err)
	}

	// Setup APIServer TLS profile watcher
	// When the cluster's TLS security profile changes, enqueue TektonConfig for reconciliation
	const skipAPIServerWatch = "SKIP_APISERVER_TLS_WATCH"
	if err := setupAPIServerTLSWatch(ctx, ctrl); err != nil {
		// On OpenShift clusters the APIServer resource should always exist.
		// This env var is an escape hatch for edge cases and must be explicitly enabled.
		if os.Getenv(skipAPIServerWatch) == "true" {
			logger.Warnf("APIServer TLS profile watch not enabled: %v", err)
		} else {
			logger.Panicf("Couldn't setup APIServer TLS profile watch: %v", err)
		}
	}

	return ctrl
}

// setupAPIServerTLSWatch sets up a watch on the OpenShift APIServer resource
// to monitor TLS security profile changes. When changes are detected, it enqueues
// TektonConfig for reconciliation so TLS config can be propagated to components.
func setupAPIServerTLSWatch(ctx context.Context, impl *controller.Impl) error {
	logger := logging.FromContext(ctx)
	restConfig := injection.GetConfig(ctx)

	// Create OpenShift config client
	configClient, err := openshiftconfigclient.NewForConfig(restConfig)
	if err != nil {
		return err
	}

	// Check if we can access the APIServer resource
	_, err = configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Create a shared informer factory for OpenShift config resources.
	// 30 minute resync is sufficient since the APIServer resource rarely changes
	// and the watch mechanism handles real-time updates.
	configInformerFactory := configinformers.NewSharedInformerFactory(configClient, 30*time.Minute)

	// Get the APIServer informer
	apiServerInformer := configInformerFactory.Config().V1().APIServers()

	// Add event handler to watch for APIServer changes
	if _, err := apiServerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldAPIServer, ok := oldObj.(*configv1.APIServer)
			if !ok {
				return
			}
			newAPIServer, ok := newObj.(*configv1.APIServer)
			if !ok {
				return
			}

			// Check if TLS security profile actually changed
			if !tlsProfileChanged(oldAPIServer, newAPIServer) {
				return
			}

			logger.Info("APIServer TLS security profile changed, triggering TektonConfig reconciliation")
			impl.EnqueueKey(types.NamespacedName{Name: v1alpha1.ConfigResourceName})
		},
	}); err != nil {
		return err
	}

	// Start the informer factory
	configInformerFactory.Start(ctx.Done())

	// Wait for caches to sync
	if !cache.WaitForCacheSync(ctx.Done(), apiServerInformer.Informer().HasSynced) {
		logger.Warn("Failed to sync APIServer informer cache")
	}

	// Share the lister with other components so they don't need to create their own informers
	occommon.SetSharedAPIServerLister(apiServerInformer.Lister(), configClient)

	return nil
}

// tlsProfileChanged checks if the TLS security profile has changed between two APIServer resources
func tlsProfileChanged(old, new *configv1.APIServer) bool {
	oldProfile := old.Spec.TLSSecurityProfile
	newProfile := new.Spec.TLSSecurityProfile

	// Both nil - no change
	if oldProfile == nil && newProfile == nil {
		return false
	}

	// One nil, one not - changed
	if (oldProfile == nil) != (newProfile == nil) {
		return true
	}

	// Different types - changed
	if oldProfile.Type != newProfile.Type {
		return true
	}

	// For custom profiles, check the actual settings
	if oldProfile.Type == configv1.TLSProfileCustomType {
		return !customProfilesEqual(oldProfile.Custom, newProfile.Custom)
	}

	// For predefined profiles (Old, Intermediate, Modern), type change is sufficient
	return false
}

// customProfilesEqual checks if two custom TLS profiles are equal
func customProfilesEqual(old, new *configv1.CustomTLSProfile) bool {
	if old == nil && new == nil {
		return true
	}
	if (old == nil) != (new == nil) {
		return false
	}

	if old.MinTLSVersion != new.MinTLSVersion {
		return false
	}

	if len(old.Ciphers) != len(new.Ciphers) {
		return false
	}
	for i := range old.Ciphers {
		if old.Ciphers[i] != new.Ciphers[i] {
			return false
		}
	}

	return true
}
