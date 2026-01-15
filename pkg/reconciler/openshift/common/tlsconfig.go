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
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	configinformer "github.com/openshift/client-go/config/informers/externalversions"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/operator/configobserver"
	"github.com/openshift/library-go/pkg/operator/configobserver/apiserver"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resourcesynccontroller"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/logging"
)

// ========================================================================
// APIServer Listers
// ========================================================================

// APIServerListers implements configobserver.Listers for APIServer resources
// This is used by library-go's ObserveTLSSecurityProfile to observe the cluster's TLS configuration
type APIServerListers struct {
	APIServerListerValue configlistersv1.APIServerLister
}

// APIServerLister returns the APIServer lister
func (l *APIServerListers) APIServerLister() configlistersv1.APIServerLister {
	return l.APIServerListerValue
}

// ResourceSyncer returns nil as we don't use resource syncing
func (l *APIServerListers) ResourceSyncer() resourcesynccontroller.ResourceSyncer {
	return nil
}

// PreRunHasSynced returns informer sync functions
func (l *APIServerListers) PreRunHasSynced() []cache.InformerSynced {
	if l.APIServerListerValue == nil {
		return nil
	}
	return []cache.InformerSynced{
		func() bool {
			// Check if the informer has synced
			return true
		},
	}
}

// ========================================================================
// TLS Config Observation
// ========================================================================

// GetTLSConfigFromAPIServer fetches the TLS profile from the API Server configuration
// using library-go's ObserveTLSSecurityProfile function.
// This is the recommended approach for OpenShift operators using the configobserver pattern.
//
// This function:
// - Observes APIServer.spec.TLSSecurityProfile via APIServerLister().Get("cluster")
// - Converts OpenSSL cipher names to IANA names using crypto.OpenSSLToIANACipherSuites
// - Returns servingInfo.minTLSVersion and servingInfo.cipherSuites
//
// Security Note: The returned TLS configuration is derived from OpenShift's cluster-wide
// security policy, not hardcoded values. This ensures compliance with administrator-defined
// security requirements and enables Post-Quantum Cryptography readiness for OpenShift 4.22+.
func GetTLSConfigFromAPIServer(listers configobserver.Listers, recorder events.Recorder) (*tls.Config, error) {
	// Use library-go's ObserveTLSSecurityProfile to observe the APIServer TLS profile
	existingConfig := map[string]interface{}{}
	observedConfig, errs := apiserver.ObserveTLSSecurityProfile(listers, recorder, existingConfig)

	if len(errs) > 0 {
		return nil, fmt.Errorf("failed to observe TLS security profile: %v", errs)
	}

	// Extract servingInfo from observed config
	servingInfo, ok := observedConfig["servingInfo"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("servingInfo not found in observed config")
	}

	// Extract minTLSVersion (e.g., "VersionTLS12")
	minTLSVersionStr, ok := servingInfo["minTLSVersion"].(string)
	if !ok {
		return nil, fmt.Errorf("minTLSVersion not found in servingInfo")
	}

	minVersion, err := parseTLSVersion(minTLSVersionStr)
	if err != nil {
		return nil, fmt.Errorf("invalid minTLSVersion: %w", err)
	}

	// Extract cipherSuites (IANA names, already converted by library-go)
	var cipherNames []string
	if ciphersInterface, ok := servingInfo["cipherSuites"].([]interface{}); ok {
		for _, c := range ciphersInterface {
			if cipherName, ok := c.(string); ok {
				cipherNames = append(cipherNames, cipherName)
			}
		}
	}

	// Build tls.Config
	config := &tls.Config{
		MinVersion: minVersion,
		// CRITICAL for FIPS: Explicitly exclude X25519 (Go's default)
		// Use only NIST P-curves (FIPS 186-4 approved)
		CurvePreferences: []tls.CurveID{
			tls.CurveP256, // secp256r1 / NIST P-256
			tls.CurveP384, // secp384r1 / NIST P-384
			tls.CurveP521, // secp521r1 / NIST P-521
		},
	}

	// For TLS 1.3, cipher suites are not configurable in Go
	if minVersion == tls.VersionTLS13 {
		config.MaxVersion = tls.VersionTLS13
	} else if len(cipherNames) > 0 {
		// Convert IANA cipher names to Go cipher suite IDs
		// library-go has already converted OpenSSL names to IANA format
		cipherSuites := cipherSuitesFromIANANames(cipherNames)
		if len(cipherSuites) == 0 {
			return nil, fmt.Errorf("no valid cipher suites found from IANA names")
		}

		config.CipherSuites = cipherSuites
	}

	return config, nil
}

// cipherSuitesFromIANANames converts IANA cipher suite names to Go's cipher suite IDs.
// This handles the names returned by library-go's ObserveTLSSecurityProfile which are
// already converted from OpenSSL to IANA format.
func cipherSuitesFromIANANames(names []string) []uint16 {
	// Build a map of all available cipher suites (IANA names -> IDs)
	ianaMap := make(map[string]uint16)

	// Add all secure cipher suites
	for _, suite := range tls.CipherSuites() {
		ianaMap[suite.Name] = suite.ID
	}

	// Include "insecure" cipher suites in the lookup map
	// These are only used if explicitly configured by cluster admin in "Old" TLS profile
	// for legacy system compatibility. The cluster admin makes the security tradeoff decision.
	// #nosec G402 -- gosec: lookup only, not enforcement
	// CodeQL suppression: See .github/codeql/codeql-config.yml
	for _, suite := range tls.InsecureCipherSuites() {
		ianaMap[suite.Name] = suite.ID
	}

	// Convert names to IDs
	suites := make([]uint16, 0, len(names))
	for _, name := range names {
		if id, ok := ianaMap[name]; ok {
			suites = append(suites, id)
		}
	}

	return suites
}

func parseTLSVersion(version string) (uint16, error) {
	switch version {
	case "VersionTLS10", "TLSv1.0":
		return tls.VersionTLS10, nil
	case "VersionTLS11", "TLSv1.1":
		return tls.VersionTLS11, nil
	case "VersionTLS12", "TLSv1.2":
		return tls.VersionTLS12, nil
	case "VersionTLS13", "TLSv1.3":
		return tls.VersionTLS13, nil
	default:
		return 0, fmt.Errorf("unknown TLS version: %s", version)
	}
}

// ========================================================================
// Platform TLS Context Management
// ========================================================================

const (
	// EnableCentralTLSConfigEnvVar is the environment variable to enable central TLS config observation
	// Set to "true" to enable the feature (opt-in)
	EnableCentralTLSConfigEnvVar = "ENABLE_CENTRAL_TLS_CONFIG"
)

// Context keys for TLS configuration
type tlsConfigHashKey struct{}
type tlsConfigKey struct{}
type tlsEnvVarsKey struct{}

// GetTLSConfigHashFromContext retrieves the TLS config hash from context
// Returns empty string if not available (e.g., for Kubernetes platform)
func GetTLSConfigHashFromContext(ctx context.Context) string {
	if hash, ok := ctx.Value(tlsConfigHashKey{}).(string); ok {
		return hash
	}
	return ""
}

// GetTLSConfigFromContext retrieves the TLS config from context
// Returns nil if not available (e.g., for Kubernetes platform or disabled)
func GetTLSConfigFromContext(ctx context.Context) *tls.Config {
	if cfg, ok := ctx.Value(tlsConfigKey{}).(*tls.Config); ok {
		return cfg
	}
	return nil
}

// IsCentralTLSConfigEnabled checks if central TLS config observation is enabled
// via the ENABLE_CENTRAL_TLS_CONFIG environment variable.
// Returns true by default (enabled) unless explicitly set to "false" (opt-out).
func IsCentralTLSConfigEnabled() bool {
	enabled := os.Getenv(EnableCentralTLSConfigEnvVar)
	if enabled == "" {
		return true // Enabled by default (opt-out model)
	}

	// Parse as boolean
	isEnabled, err := strconv.ParseBool(enabled)
	if err != nil {
		// If invalid value, default to enabled
		return true
	}

	return isEnabled
}

// ObserveAndStoreTLSConfig observes the cluster's TLS profile and stores the hash in context
// This is called once at operator startup for OpenShift platform
// Enabled by default unless ENABLE_CENTRAL_TLS_CONFIG=false is set (opt-out)
func ObserveAndStoreTLSConfig(ctx context.Context, cfg *rest.Config) context.Context {
	logger := logging.FromContext(ctx)

	// Check if feature is enabled
	if !IsCentralTLSConfigEnabled() {
		logger.Info("Central TLS config observation is explicitly disabled via ENABLE_CENTRAL_TLS_CONFIG=false")
		return ctx
	}

	// Create config client
	configClient, err := configclient.NewForConfig(cfg)
	if err != nil {
		logger.Errorw("Failed to create OpenShift config client, TLS config will not be observed", "error", err)
		return ctx
	}

	// Create informer factory
	configInformerFactory := configinformer.NewSharedInformerFactory(configClient, 0)
	apiServerInformer := configInformerFactory.Config().V1().APIServers()

	// Add event handler before starting informer (required for proper sync)
	if _, err := apiServerInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			// Event handler helps informer establish watch
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			logger.Info("APIServer TLS profile updated - components will be updated on next reconciliation cycle")
		},
	}); err != nil {
		logger.Errorw("Failed to add event handler to APIServer informer", "error", err)
		return ctx
	}

	// Start informer and wait for cache sync with timeout
	configInformerFactory.Start(ctx.Done())

	syncCtx, syncCancel := context.WithTimeout(ctx, 30*time.Second)
	defer syncCancel()

	if !cache.WaitForCacheSync(syncCtx.Done(), apiServerInformer.Informer().HasSynced) {
		logger.Error("Failed to sync APIServer informer cache within timeout, TLS config will not be observed. Check RBAC permissions for config.openshift.io/apiservers")
		return ctx
	}

	// Create listers for library-go's ObserveTLSSecurityProfile
	listers := &APIServerListers{
		APIServerListerValue: apiServerInformer.Lister(),
	}
	recorder := events.NewLoggingEventRecorder("tekton-operator")

	// Get initial TLS config
	tlsConfig, err := GetTLSConfigFromAPIServer(listers, recorder)
	if err != nil {
		logger.Errorw("Failed to get TLS config from APIServer", "error", err)
		return ctx
	}

	// Calculate hash of TLS config
	tlsConfigHash := CalculateTLSConfigHash(tlsConfig)

	// Convert to environment variables
	tlsEnvVars := TLSEnvVarsFromConfig(tlsConfig)

	logger.Infof("TLS config observed from cluster: MinVersion=%s, CipherSuites=%d, Hash=%s",
		tlsEnvVars.MinVersion, len(tlsConfig.CipherSuites), tlsConfigHash)

	// Store config, hash, and env vars in context for all reconcilers to use
	ctx = context.WithValue(ctx, tlsConfigKey{}, tlsConfig)
	ctx = context.WithValue(ctx, tlsConfigHashKey{}, tlsConfigHash)
	ctx = context.WithValue(ctx, tlsEnvVarsKey{}, tlsEnvVars)

	return ctx
}

// CalculateTLSConfigHash creates a deterministic hash of the TLS configuration
func CalculateTLSConfigHash(tlsConfig *tls.Config) string {
	if tlsConfig == nil {
		return ""
	}

	// Build deterministic string representation of TLS config
	var parts []string

	// Add MinVersion
	parts = append(parts, TLSVersionToString(tlsConfig.MinVersion))

	// Add cipher suites (sorted for determinism)
	cipherNames := CipherSuitesToNames(tlsConfig.CipherSuites)
	sort.Strings(cipherNames)
	parts = append(parts, strings.Join(cipherNames, ","))

	// Add curves (always P-256, P-384, P-521 for FIPS compliance)
	parts = append(parts, "P-256,P-384,P-521")

	// Calculate SHA256 hash
	configString := strings.Join(parts, "|")
	hash := sha256.Sum256([]byte(configString))
	return hex.EncodeToString(hash[:])[:16] // Use first 16 chars for brevity
}

// TLSVersionToString converts tls.Version constants to string
// This is exported for use by reconcilers creating TLS ConfigMaps
func TLSVersionToString(version uint16) string {
	switch version {
	case tls.VersionTLS10:
		return "TLSv1.0"
	case tls.VersionTLS11:
		return "TLSv1.1"
	case tls.VersionTLS12:
		return "TLSv1.2"
	case tls.VersionTLS13:
		return "TLSv1.3"
	default:
		return "TLSv1.2" // safe default
	}
}

// CipherSuitesToNames converts cipher suite IDs to IANA names
// This is exported for use by reconcilers creating TLS ConfigMaps
func CipherSuitesToNames(suites []uint16) []string {
	if len(suites) == 0 {
		return []string{}
	}

	var names []string

	// Build map of all cipher suites for ID-to-name conversion
	suiteMap := make(map[uint16]string)
	for _, suite := range tls.CipherSuites() {
		suiteMap[suite.ID] = suite.Name
	}
	// Include "insecure" ciphers in map for name lookup
	// These are only converted if present in cluster TLS config
	// #nosec G402 -- gosec: read-only lookup, not configuration
	// CodeQL suppression: See .github/codeql/codeql-config.yml
	for _, suite := range tls.InsecureCipherSuites() {
		suiteMap[suite.ID] = suite.Name
	}

	// Convert IDs to names
	for _, id := range suites {
		if name, ok := suiteMap[id]; ok {
			names = append(names, name)
		}
	}

	return names
}

// CurveIDsToString converts tls.CurveID slice to comma-separated string
// e.g., [tls.CurveP256, tls.CurveP384] → "P-256,P-384"
func CurveIDsToString(curves []tls.CurveID) string {
	if len(curves) == 0 {
		return ""
	}

	names := make([]string, 0, len(curves))
	for _, curve := range curves {
		switch curve {
		case tls.CurveP256:
			names = append(names, "P-256")
		case tls.CurveP384:
			names = append(names, "P-384")
		case tls.CurveP521:
			names = append(names, "P-521")
		case tls.X25519:
			names = append(names, "X25519")
		default:
			// Unknown curve, use numeric representation
			names = append(names, fmt.Sprintf("curve-%d", uint16(curve)))
		}
	}
	return strings.Join(names, ",")
}

// ========================================================================
// TLS Environment Variables for Components
// ========================================================================

// TLSEnvVarNames defines the environment variable names for TLS configuration.
// Components should use these constants for consistency.
const (
	TLSMinVersionEnvVar       = "TLS_MIN_VERSION"
	TLSCipherSuitesEnvVar     = "TLS_CIPHER_SUITES"
	TLSCurvePreferencesEnvVar = "TLS_CURVE_PREFERENCES"
)

// TLSEnvVars represents the TLS configuration as environment variable values
// that can be injected into component pods.
// All values are human-readable strings for easier debugging and operations.
type TLSEnvVars struct {
	// MinVersion contains the minimum TLS version (e.g., "TLSv1.2", "TLSv1.3")
	MinVersion string

	// CipherSuites contains comma-separated IANA cipher suite names
	// e.g., "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384,TLS_CHACHA20_POLY1305_SHA256"
	CipherSuites string

	// CurvePreferences contains comma-separated elliptic curve names
	// e.g., "P-256,P-384,P-521"
	// This is critical for PQC (Post-Quantum Cryptography) readiness
	CurvePreferences string
}

// TLSEnvVarsFromConfig converts a native tls.Config to TLSEnvVars
// This performs the conversion from Go types to human-readable strings
func TLSEnvVarsFromConfig(cfg *tls.Config) *TLSEnvVars {
	if cfg == nil {
		return nil
	}

	return &TLSEnvVars{
		MinVersion:       TLSVersionToString(cfg.MinVersion),
		CipherSuites:     strings.Join(CipherSuitesToNames(cfg.CipherSuites), ","),
		CurvePreferences: CurveIDsToString(cfg.CurvePreferences),
	}
}

// GetTLSEnvVarsFromContext retrieves TLS config from context and converts to env vars
// Returns nil if TLS config is not available
func GetTLSEnvVarsFromContext(ctx context.Context) *TLSEnvVars {
	tlsConfig := GetTLSConfigFromContext(ctx)
	if tlsConfig == nil {
		return nil
	}
	return TLSEnvVarsFromConfig(tlsConfig)
}
