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
	cryptotls "crypto/tls"
	"errors"
	"fmt"
	"strings"
	"sync"

	mf "github.com/manifestival/manifestival"
	configv1 "github.com/openshift/api/config/v1"
	openshiftconfigclient "github.com/openshift/client-go/config/clientset/versioned"
	configv1listers "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/crypto"
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
	// TLS_MIN_VERSION: Go crypto/tls version string, e.g. "1.0", "1.1", "1.2", "1.3"
	TLSMinVersionEnvVar = "TLS_MIN_VERSION"
	// TLS_CIPHER_SUITES: comma-separated IANA cipher suite names,
	// e.g. "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_AES_128_GCM_SHA256"
	TLSCipherSuitesEnvVar = "TLS_CIPHER_SUITES"
	// TLS_CURVE_PREFERENCES: comma-separated curve names for Go/Knative TLS
	// (e.g. "X25519MLKEM768,X25519,P-256,P-384"). OpenShift API TLSGroup names
	// (secp256r1) are converted before injection — knative.dev/pkg rejects API names.
	TLSCurvePreferencesEnvVar = "TLS_CURVE_PREFERENCES"

	// WebhookEnvVarPrefix is prepended to the TLS env var names when injecting into
	// deployments that use the Knative webhook framework. The Knative webhook binary calls
	// knativetls.DefaultConfigFromEnv("WEBHOOK_"), so it reads WEBHOOK_TLS_MIN_VERSION,
	// WEBHOOK_TLS_CIPHER_SUITES, and WEBHOOK_TLS_CURVE_PREFERENCES.
	// Components that read the env vars directly (e.g. Results API) use no prefix.
	WebhookEnvVarPrefix = "WEBHOOK_"
)

// TLSEnvVars holds TLS configuration as environment variable values
type TLSEnvVars struct {
	MinVersion       string
	CipherSuites     string
	CurvePreferences string
}

// noOpRecorder discards all events. ObserveTLSSecurityProfile requires a non-nil
// recorder and logs "changed" events by comparing against the existingConfig we
// pass in. Because we always pass an empty existingConfig (we only need the
// return value, not the diff), the recorder would emit spurious "minTLSVersion
// changed" / "cipherSuites changed" messages on every call. A no-op recorder
// silences this noise; actual TLS change detection is handled by the APIServer
// informer event handler in the TektonConfig controller.
type noOpRecorder struct{}

func (noOpRecorder) Event(string, string)                        {}
func (noOpRecorder) Eventf(string, string, ...interface{})       {}
func (noOpRecorder) Warning(string, string)                      {}
func (noOpRecorder) Warningf(string, string, ...interface{})     {}
func (noOpRecorder) ForComponent(string) events.Recorder         { return noOpRecorder{} }
func (noOpRecorder) WithComponentSuffix(string) events.Recorder  { return noOpRecorder{} }
func (noOpRecorder) WithContext(context.Context) events.Recorder { return noOpRecorder{} }
func (noOpRecorder) ComponentName() string                       { return "" }
func (noOpRecorder) Shutdown()                                   {}

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
// Values are in library-go / OpenShift API format (e.g. "VersionTLS12", IANA cipher names,
// TLSGroup names such as "X25519MLKEM768" / "secp256r1").
type TLSProfileConfig struct {
	MinTLSVersion    string
	CipherSuites     []string
	CurvePreferences []string
}

// GetTLSProfileFromAPIServer fetches the raw TLS security profile from the OpenShift APIServer
// resource. Returns (nil, nil) if no TLS profile is configured or the shared lister is not initialized.
func GetTLSProfileFromAPIServer(ctx context.Context) (*TLSProfileConfig, error) {
	logger := logging.FromContext(ctx)

	sharedListerMu.RLock()
	lister := sharedAPIServerLister
	sharedListerMu.RUnlock()

	if lister == nil {
		logger.Debug("Shared APIServer lister not initialized, TLS config not available")
		return nil, nil
	}

	listers := &APIServerListers{
		lister: lister,
	}

	// Use library-go's ObserveTLSSecurityProfileWithGroupPaths to extract minTLSVersion,
	// cipherSuites, and TLS group (curve) preferences from the cluster APIServer profile.
	// Requires a non-nil recorder (for Eventf calls) and a non-nil existingConfig
	// (read via unstructured.NestedString).
	// The returned cipher list includes both TLS 1.2 and TLS 1.3 IANA names because
	// library-go's OpenSSLToIANACipherSuites maps TLS 1.3 names (TLS_AES_*,
	// TLS_CHACHA20_POLY1305_SHA256) as identity values.
	// Groups are FIPS-filtered by library-go when the Go runtime is in FIPS mode.
	existingConfig := map[string]interface{}{}
	observedConfig, errs := apiserver.ObserveTLSSecurityProfileWithGroupPaths(
		listers,
		noOpRecorder{},
		existingConfig,
		[]string{"servingInfo", "minTLSVersion"},
		[]string{"servingInfo", "cipherSuites"},
		[]string{"servingInfo", "curvePreferences"},
	)
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

	var curvePreferences []string
	if groups, ok := servingInfo["curvePreferences"].([]interface{}); ok {
		for _, g := range groups {
			if gs, ok := g.(string); ok {
				curvePreferences = append(curvePreferences, gs)
			}
		}
	}

	if minVersion == "" && len(cipherSuites) == 0 && len(curvePreferences) == 0 {
		return nil, nil
	}

	return &TLSProfileConfig{
		MinTLSVersion:    minVersion,
		CipherSuites:     cipherSuites,
		CurvePreferences: curvePreferences,
	}, nil
}

// TLSEnvVarsFromProfile validates and converts a raw TLSProfileConfig to TLSEnvVars
// suitable for injection into component deployments.
//
// CurvePreferences are remapped from OpenShift API TLSGroup names (secp256r1)
// to the names accepted by knative.dev/pkg network/tls (P-256, X25519, …).
// Injecting API names verbatim crashes Knative webhooks with:
//
//	invalid WEBHOOK_TLS_CURVE_PREFERENCES: unknown curve "secp256r1"
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
		CurvePreferences: joinKnativeCurvePreferences(cfg.CurvePreferences),
	}, nil
}

// joinKnativeCurvePreferences converts OpenShift API TLSGroup names to a
// comma-separated list accepted by knative.dev/pkg/network/tls parseCurvePreferences
// (P-256 / X25519 / X25519MLKEM768, …). Unknown groups are dropped so a future
// API-only name cannot CrashLoop the webhook.
func joinKnativeCurvePreferences(apiGroups []string) string {
	out := make([]string, 0, len(apiGroups))
	for _, g := range apiGroups {
		if name, ok := apiTLSGroupToKnativeCurve(g); ok {
			out = append(out, name)
		}
	}
	return strings.Join(out, ",")
}

// knativeCurveNameByID maps Go CurveIDs to the env-string form Knative accepts.
// Prefer the P-* / X25519 names from knative.dev/pkg/network/tls curvesByName
// (https://github.com/knative/pkg/blob/main/network/tls/config.go).
// SecP*MLKEM* IDs are omitted until Knative lists them.
var knativeCurveNameByID = map[cryptotls.CurveID]string{
	cryptotls.CurveP256:      "P-256",
	cryptotls.CurveP384:      "P-384",
	cryptotls.CurveP521:      "P-521",
	cryptotls.X25519:         "X25519",
	cryptotls.X25519MLKEM768: "X25519MLKEM768",
}

// knativeCurveByName mirrors knative.dev/pkg/network/tls curvesByName so that
// values already in Knative form (P-256, CurveP256) normalize correctly.
var knativeCurveByName = map[string]cryptotls.CurveID{
	"CurveP256":      cryptotls.CurveP256,
	"CurveP384":      cryptotls.CurveP384,
	"CurveP521":      cryptotls.CurveP521,
	"X25519":         cryptotls.X25519,
	"X25519MLKEM768": cryptotls.X25519MLKEM768,
	"P-256":          cryptotls.CurveP256,
	"P-384":          cryptotls.CurveP384,
	"P-521":          cryptotls.CurveP521,
}

// apiTLSGroupToKnativeCurve maps an OpenShift API TLSGroup (or a Knative alias)
// to a name in knative curvesByName.
//
// Path: API string → library-go TLSGroupToCurveID → knativeCurveNameByID.
// If the string is already a Knative alias, resolve via knativeCurveByName.
func apiTLSGroupToKnativeCurve(group string) (string, bool) {
	group = strings.TrimSpace(group)
	if group == "" {
		return "", false
	}
	if id, ok := crypto.TLSGroupToCurveID(configv1.TLSGroup(group)); ok {
		name, ok := knativeCurveNameByID[id]
		return name, ok
	}
	if id, ok := knativeCurveByName[group]; ok {
		name, ok := knativeCurveNameByID[id]
		return name, ok
	}
	return "", false
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
		return nil, err
	}

	// nil means the field was not set → treat as true (default-on after SetDefaults).
	// Explicitly false means the user opted out.
	if tc.Spec.Platforms.OpenShift.EnableCentralTLSConfig != nil &&
		!*tc.Spec.Platforms.OpenShift.EnableCentralTLSConfig {
		return nil, nil
	}

	// Note: GetTLSProfileFromAPIServer returns the cluster's effective TLS profile,
	// which currently defaults to the Intermediate profile when no explicit
	// .spec.tlsSecurityProfile is set on the APIServer resource. This is consistent
	// with library-go's ObserveTLSSecurityProfileWithGroupPaths behavior used by
	// other OpenShift components.
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
// envVarPrefix is prepended to each env var name. Use WebhookEnvVarPrefix ("WEBHOOK_") for
// deployments that run the Knative webhook binary, and "" for all other components.
func InjectTLSEnvVars(tlsEnvVars *TLSEnvVars, kind string, resourceName string, containerNames []string, envVarPrefix string) mf.Transformer {
	return func(u *unstructured.Unstructured) error {
		if u.GetKind() != kind || u.GetName() != resourceName {
			return nil
		}

		envVars := buildTLSEnvVarList(tlsEnvVars, envVarPrefix)
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

func buildTLSEnvVarList(tlsEnvVars *TLSEnvVars, prefix string) []corev1.EnvVar {
	var envVars []corev1.EnvVar
	if tlsEnvVars.MinVersion != "" {
		envVars = append(envVars, corev1.EnvVar{Name: prefix + TLSMinVersionEnvVar, Value: tlsEnvVars.MinVersion})
	}
	if tlsEnvVars.CipherSuites != "" {
		envVars = append(envVars, corev1.EnvVar{Name: prefix + TLSCipherSuitesEnvVar, Value: tlsEnvVars.CipherSuites})
	}
	if tlsEnvVars.CurvePreferences != "" {
		envVars = append(envVars, corev1.EnvVar{Name: prefix + TLSCurvePreferencesEnvVar, Value: tlsEnvVars.CurvePreferences})
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
