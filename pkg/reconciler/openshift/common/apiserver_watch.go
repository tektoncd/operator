/*
Copyright 2025 The Tekton Authors

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

package common

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	openshiftconfigclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// ResourceLister is an interface for listing Kubernetes resources
// This allows the watch to work with any Tekton component type
type ResourceLister interface {
	List(selector labels.Selector) ([]ResourceWithName, error)
}

// ResourceWithName is an interface for resources that have a name
type ResourceWithName interface {
	GetName() string
}

// SetupAPIServerTLSWatch sets up a watch on the OpenShift APIServer resource
// to monitor TLS security profile changes. When changes are detected, it enqueues
// the component's resources for reconciliation. Returns an error only for unexpected
// failures; returns nil if APIServer is unavailable.
func SetupAPIServerTLSWatch(
	ctx context.Context,
	restConfig *rest.Config,
	impl *controller.Impl,
	lister ResourceLister,
	componentName string,
) error {
	logger := logging.FromContext(ctx).With("component", componentName)

	// Create OpenShift config client
	configClient, err := openshiftconfigclient.NewForConfig(restConfig)
	if err != nil {
		logger.Errorf("Failed to create OpenShift config client: %v", err)
		return fmt.Errorf("failed to create OpenShift config client: %w", err)
	}

	// Check if we can access the APIServer resource
	_, err = configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// APIServer resource doesn't exist - TLS watch is not available
			logger.Info("APIServer 'cluster' resource not found, TLS profile watch disabled")
			return nil
		}
		// Real error - log and return
		logger.Errorf("Failed to access APIServer resource: %v", err)
		return fmt.Errorf("failed to access APIServer resource: %w", err)
	}

	// Create a shared informer factory for OpenShift config resources
	configInformerFactory := configinformers.NewSharedInformerFactory(configClient, 10*time.Minute)

	// Get the APIServer informer
	apiServerInformer := configInformerFactory.Config().V1().APIServers()

	// Add event handler to watch for APIServer changes
	if _, err := apiServerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldAPIServer, ok := oldObj.(*configv1.APIServer)
			if !ok {
				logger.Warn("Failed to cast old object to APIServer")
				return
			}
			newAPIServer, ok := newObj.(*configv1.APIServer)
			if !ok {
				logger.Warn("Failed to cast new object to APIServer")
				return
			}

			// Check if TLS security profile actually changed
			if !tlsProfileChanged(oldAPIServer, newAPIServer) {
				logger.Debug("APIServer updated but TLS profile unchanged, skipping reconciliation")
				return
			}

			logger.Infof("APIServer TLS security profile changed, triggering %s reconciliation", componentName)

			resources, err := lister.List(labels.Everything())
			if err != nil {
				logger.Errorf("Failed to list %s resources after APIServer change: %v", componentName, err)
				return
			}

			for _, resource := range resources {
				logger.Infof("Enqueuing %s %s for reconciliation due to APIServer TLS change",
					componentName, resource.GetName())
				impl.EnqueueKey(types.NamespacedName{Name: resource.GetName()})
			}
		},
	}); err != nil {
		return fmt.Errorf("failed to add APIServer event handler: %w", err)
	}

	// Start the informer factory
	configInformerFactory.Start(ctx.Done())

	// Wait for caches to sync
	logger.Info("Waiting for APIServer informer cache to sync...")
	if !cache.WaitForCacheSync(ctx.Done(), apiServerInformer.Informer().HasSynced) {
		return fmt.Errorf("failed to wait for APIServer informer cache to sync")
	}
	logger.Info("APIServer informer cache synced successfully")

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

// customProfilesEqual checks if two custom TLS profiles are equal.
// TODO(openshift/api#2583): Add curve preferences comparison once the field is added to TLSProfileSpec.
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
