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
	"errors"
	"fmt"
	"strings"
	"sync"

	mf "github.com/manifestival/manifestival"
	configv1 "github.com/openshift/api/config/v1"
	openshiftconfigclient "github.com/openshift/client-go/config/clientset/versioned"
	configv1listers "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/operator/configobserver/apiserver"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/logging"

	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
)

const (
	// TLS environment variable names used by Tekton components
	TLSMinVersionEnvVar       = "TLS_MIN_VERSION"
	TLSCipherSuitesEnvVar     = "TLS_CIPHER_SUITES"
	TLSCurvePreferencesEnvVar = "TLS_CURVE_PREFERENCES"
)

// TLSEnvVars holds TLS configuration as environment variable values
type TLSEnvVars struct {
	MinVersion       string
	CipherSuites     string
	CurvePreferences string
}

// APIServerListers implements the configobserver.Listers interface for accessing APIServer resources.
// This adapter enables using library-go's ObserveTLSSecurityProfile function with our informer setup.
type APIServerListers struct {
	lister configv1listers.APIServerLister
}

// APIServerLister returns the APIServer lister
func (a *APIServerListers) APIServerLister() configv1listers.APIServerLister {
	return a.lister
}

// ResourceSyncer is not used but required by the Listers interface
func (a *APIServerListers) ResourceSyncer() resourcesynccontroller.ResourceSyncer {
	return nil
}

// PreRunHasSynced returns nil (no pre-run sync needed)
func (a *APIServerListers) PreRunHasSynced() []cache.InformerSynced {
	return nil
}

// sharedAPIServerLister holds the singleton lister and client for APIServer resources.
// This is initialized once by the TektonConfig controller and shared across all components.
var (
	sharedAPIServerLister configv1listers.APIServerLister
	sharedConfigClient    openshiftconfigclient.Interface
	sharedListerMu        sync.RWMutex
)

// SetSharedAPIServerLister sets the shared APIServer lister and client.
// This should be called once during TektonConfig controller initialization.
func SetSharedAPIServerLister(lister configv1listers.APIServerLister, client openshiftconfigclient.Interface) {
	sharedListerMu.Lock()
	defer sharedListerMu.Unlock()
	sharedAPIServerLister = lister
	sharedConfigClient = client
}

// TLSProfileConfig holds the raw TLS profile data as extracted from the APIServer resource.
// Values are in library-go / OpenShift API format (e.g. "VersionTLS12", IANA cipher names).
type TLSProfileConfig struct {
	MinTLSVersion    string
	CipherSuites     []string
	CurvePreferences []string // Not yet populated; will be set once openshift/api#2583 is merged
}

// GetTLSProfileFromAPIServer fetches the raw TLS security profile from the OpenShift APIServer
// resource. Returns (nil, nil) if no TLS profile is configured or the shared lister is not initialized.
func GetTLSProfileFromAPIServer(ctx context.Context) (*TLSProfileConfig, error) {
	logger := logging.FromContext(ctx)

	sharedListerMu.RLock()
	lister := sharedAPIServerLister
	client := sharedConfigClient
	sharedListerMu.RUnlock()

	if lister == nil {
		logger.Debug("Shared APIServer lister not initialized, TLS config not available")
		return nil, nil
	}

	listers := &APIServerListers{
		lister: lister,
	}

	// Use library-go's ObserveTLSSecurityProfile to extract TLS config.
	// Note: ObserveTLSSecurityProfile requires:
	// - non-nil recorder: it calls recorder.Eventf() to log changes
	// - non-nil existingConfig: it reads from it via unstructured.NestedString()
	// TODO: Once library-go is updated to a newer version (with TLS 1.3 cipher support),
	// the supplementTLS13Ciphers workaround below can be removed.
	existingConfig := map[string]interface{}{}
	recorder := events.NewLoggingEventRecorder("tekton-operator")
	observedConfig, errs := apiserver.ObserveTLSSecurityProfile(listers, recorder, existingConfig)
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}

	servingInfo, ok := observedConfig["servingInfo"].(map[string]interface{})
	if !ok {
		return nil, nil
	}

	minVersion, _ := servingInfo["minTLSVersion"].(string)

	var cipherSuites []string
	if ciphers, ok := servingInfo["cipherSuites"].([]interface{}); ok {
		for _, c := range ciphers {
			if cs, ok := c.(string); ok {
				cipherSuites = append(cipherSuites, cs)
			}
		}
	}

	if minVersion == "" && len(cipherSuites) == 0 {
		return nil, nil
	}

	// Supplement TLS 1.3 ciphers if needed
	// TODO: Remove this once library-go is updated with proper TLS 1.3 cipher mapping
	if client != nil {
		apiServer, err := lister.Get("cluster")
		if err == nil && apiServer.Spec.TLSSecurityProfile != nil {
			cipherSuites = supplementTLS13Ciphers(apiServer.Spec.TLSSecurityProfile, cipherSuites)
		}
	}

	return &TLSProfileConfig{
		MinTLSVersion:    minVersion,
		CipherSuites:     cipherSuites,
		CurvePreferences: nil,
	}, nil
}

// TLSEnvVarsFromProfile validates and converts a raw TLSProfileConfig to TLSEnvVars
// suitable for injection into component deployments.
func TLSEnvVarsFromProfile(cfg *TLSProfileConfig) (*TLSEnvVars, error) {
	if cfg == nil {
		return nil, nil
	}

	envMinVersion, err := convertTLSVersionToEnvFormat(cfg.MinTLSVersion)
	if err != nil {
		return nil, fmt.Errorf("invalid TLS configuration: %w", err)
	}

	return &TLSEnvVars{
		MinVersion:       envMinVersion,
		CipherSuites:     strings.Join(cfg.CipherSuites, ","),
		CurvePreferences: strings.Join(cfg.CurvePreferences, ","),
	}, nil
}

// TektonConfigLister abstracts access to TektonConfig resources.
type TektonConfigLister interface {
	Get(name string) (*v1alpha1.TektonConfig, error)
}

// ResolveCentralTLSToEnvVars checks whether central TLS config is enabled in TektonConfig,
// fetches the raw profile from the shared APIServer lister, and converts it to env vars.
// Returns (nil, nil) if central TLS is disabled or no TLS config is available.
func ResolveCentralTLSToEnvVars(ctx context.Context, lister TektonConfigLister) (*TLSEnvVars, error) {
	tc, err := lister.Get(v1alpha1.ConfigResourceName)
	if err != nil {
		return nil, nil
	}

	if !tc.Spec.Platforms.OpenShift.EnableCentralTLSConfig {
		return nil, nil
	}

	profile, err := GetTLSProfileFromAPIServer(ctx)
	if err != nil || profile == nil {
		return nil, err
	}
	return TLSEnvVarsFromProfile(profile)
}

// convertTLSVersionToEnvFormat converts library-go TLS version format (VersionTLSxx) to
// the format expected by Go's crypto/tls (1.x)
func convertTLSVersionToEnvFormat(version string) (string, error) {
	switch version {
	case "VersionTLS10":
		return "1.0", nil
	case "VersionTLS11":
		return "1.1", nil
	case "VersionTLS12":
		return "1.2", nil
	case "VersionTLS13":
		return "1.3", nil
	default:
		return "", fmt.Errorf("unknown TLS version: %s", version)
	}
}

// InjectTLSEnvVars returns a transformer that injects TLS environment variables into
// the specified containers of a Deployment or StatefulSet matched by name.
func InjectTLSEnvVars(tlsEnvVars *TLSEnvVars, kind string, resourceName string, containerNames []string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != kind || u.GetName() != resourceName {
			return nil
		}

		envVars := buildTLSEnvVarList(tlsEnvVars)
		if len(envVars) == 0 {
			return nil
		}

		switch kind {
		case "Deployment":
			return injectEnvVarsIntoDeployment(u, containerNames, envVars)
		case "StatefulSet":
			return injectEnvVarsIntoStatefulSet(u, containerNames, envVars)
		}
		return nil
	}
}

func buildTLSEnvVarList(tlsEnvVars *TLSEnvVars) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	if tlsEnvVars.MinVersion != "" {
		envVars = append(envVars, corev1.EnvVar{Name: TLSMinVersionEnvVar, Value: tlsEnvVars.MinVersion})
	}
	if tlsEnvVars.CipherSuites != "" {
		envVars = append(envVars, corev1.EnvVar{Name: TLSCipherSuitesEnvVar, Value: tlsEnvVars.CipherSuites})
	}
	if tlsEnvVars.CurvePreferences != "" {
		envVars = append(envVars, corev1.EnvVar{Name: TLSCurvePreferencesEnvVar, Value: tlsEnvVars.CurvePreferences})
	}
	return envVars
}

func injectEnvVarsIntoDeployment(u *unstructured.Unstructured, containerNames []string, envVars []corev1.EnvVar) error {
	d := &appsv1.Deployment{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, d); err != nil {
		return err
	}
	mergeEnvVarsIntoContainers(d.Spec.Template.Spec.Containers, containerNames, envVars)
	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(d)
	if err != nil {
		return err
	}
	u.SetUnstructuredContent(uObj)
	return nil
}

func injectEnvVarsIntoStatefulSet(u *unstructured.Unstructured, containerNames []string, envVars []corev1.EnvVar) error {
	sts := &appsv1.StatefulSet{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, sts); err != nil {
		return err
	}
	mergeEnvVarsIntoContainers(sts.Spec.Template.Spec.Containers, containerNames, envVars)
	uObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(sts)
	if err != nil {
		return err
	}
	u.SetUnstructuredContent(uObj)
	return nil
}

func mergeEnvVarsIntoContainers(containers []corev1.Container, names []string, envVars []corev1.EnvVar) {
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}
	for i, container := range containers {
		if !nameSet[container.Name] {
			continue
		}
		existing := container.Env
		for _, newEnv := range envVars {
			found := false
			for j, e := range existing {
				if e.Name == newEnv.Name {
					existing[j] = newEnv
					found = true
					break
				}
			}
			if !found {
				existing = append(existing, newEnv)
			}
		}
		containers[i].Env = existing
	}
}

// supplementTLS13Ciphers adds TLS 1.3 ciphers that the older library-go version doesn't map.
// TLS 1.3 ciphers are mandatory per RFC 8446 and are always enabled when TLS 1.3 is used,
// but we include them explicitly for completeness.
// TODO: Remove this function once library-go is updated to a version that properly maps TLS 1.3 ciphers.
func supplementTLS13Ciphers(profile *configv1.TLSSecurityProfile, observedCiphers []string) []string {
	if profile == nil {
		return observedCiphers
	}

	// Get the profile spec that defines the configured ciphers
	var profileSpec *configv1.TLSProfileSpec
	switch profile.Type {
	case configv1.TLSProfileCustomType:
		if profile.Custom != nil {
			profileSpec = &profile.Custom.TLSProfileSpec
		}
	case configv1.TLSProfileModernType:
		profileSpec = configv1.TLSProfiles[configv1.TLSProfileModernType]
	case configv1.TLSProfileIntermediateType:
		profileSpec = configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
	case configv1.TLSProfileOldType:
		profileSpec = configv1.TLSProfiles[configv1.TLSProfileOldType]
	}

	if profileSpec == nil {
		return observedCiphers
	}

	// Build a set of already observed ciphers for quick lookup
	observedSet := make(map[string]bool)
	for _, c := range observedCiphers {
		observedSet[c] = true
	}

	// TLS 1.3 cipher suite names (IANA names)
	tls13Ciphers := map[string]bool{
		"TLS_AES_128_GCM_SHA256":       true,
		"TLS_AES_256_GCM_SHA384":       true,
		"TLS_CHACHA20_POLY1305_SHA256": true,
	}

	// Check configured ciphers for TLS 1.3 ciphers that library-go might have missed
	result := observedCiphers
	for _, cipher := range profileSpec.Ciphers {
		// If it's a TLS 1.3 cipher and not already in observed list, add it
		if tls13Ciphers[cipher] && !observedSet[cipher] {
			result = append(result, cipher)
		}
	}

	return result
}
