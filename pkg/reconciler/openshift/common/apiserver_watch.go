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

package common

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	openshiftconfigclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformers "github.com/openshift/client-go/config/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/logging"
)

// SkipAPIServerTLSWatch is the env var name used as an escape hatch to suppress
// a fatal error when the APIServer resource is unreachable (e.g. in tests).
// Both the operator controller and the webhook check this variable.
const SkipAPIServerTLSWatch = "SKIP_APISERVER_TLS_WATCH"

// SetupAPIServerTLSWatch creates an OpenShift APIServer informer, registers
// onTLSChange to be called whenever the TLS security profile changes, waits for
// the informer cache to sync, and then sets the shared APIServer lister so that
// GetTLSProfileFromAPIServer works in the calling process.
//
// This function is intentionally generic so it can be used by any process:
//   - The operator controller passes impl.EnqueueKey(...) as onTLSChange.
//   - The webhook binary passes os.Exit(1) as onTLSChange (restarts to pick up new TLS config).
func SetupAPIServerTLSWatch(ctx context.Context, cfg *rest.Config, onTLSChange func()) error {
	logger := logging.FromContext(ctx)

	configClient, err := openshiftconfigclient.NewForConfig(cfg)
	if err != nil {
		return fmt.Errorf("creating OpenShift config client: %w", err)
	}

	// Verify we can access the APIServer resource before starting the informer.
	if _, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{}); err != nil {
		return fmt.Errorf("accessing APIServer resource: %w", err)
	}

	// 30-minute resync is sufficient; the watch mechanism handles real-time updates.
	factory := configinformers.NewSharedInformerFactory(configClient, 30*time.Minute)
	apiServerInformer := factory.Config().V1().APIServers()

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
			if !APIServerTLSProfileChanged(oldAPIServer, newAPIServer) {
				return
			}
			logger.Info("APIServer TLS security profile changed")
			onTLSChange()
		},
	}); err != nil {
		return fmt.Errorf("adding APIServer event handler: %w", err)
	}

	factory.Start(ctx.Done())

	if !cache.WaitForCacheSync(ctx.Done(), apiServerInformer.Informer().HasSynced) {
		return fmt.Errorf("failed to sync APIServer informer cache")
	}

	// Populate the shared lister so GetTLSProfileFromAPIServer works in this process.
	SetSharedAPIServerLister(apiServerInformer.Lister(), configClient)
	return nil
}

// APIServerTLSProfileChanged reports whether the TLS security profile has changed
// between two APIServer resources.
func APIServerTLSProfileChanged(old, new *configv1.APIServer) bool {
	oldProfile := old.Spec.TLSSecurityProfile
	newProfile := new.Spec.TLSSecurityProfile

	if oldProfile == nil && newProfile == nil {
		return false
	}
	if (oldProfile == nil) != (newProfile == nil) {
		return true
	}
	if oldProfile.Type != newProfile.Type {
		return true
	}
	if oldProfile.Type == configv1.TLSProfileCustomType {
		return !customTLSProfilesEqual(oldProfile.Custom, newProfile.Custom)
	}
	return false
}

// customTLSProfilesEqual compares two custom TLS profile specs for equality.
func customTLSProfilesEqual(old, new *configv1.CustomTLSProfile) bool {
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
