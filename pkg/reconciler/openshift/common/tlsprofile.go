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
	"sort"
	"strings"

	configv1 "github.com/openshift/api/config/v1"
	openshiftconfigclient "github.com/openshift/client-go/config/clientset/versioned"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"knative.dev/pkg/logging"
)

// TLSEnvVars represents the TLS configuration extracted from OpenShift's APIServer
// that can be injected into components' deployment as environment variables.
type TLSEnvVars struct {
	MinVersion   string // e.g., "1.2", "1.3"
	CipherSuites string // comma-separated IANA cipher suite names
	// CurvePreferences will be populated once openshift/api#2583 is merged,
	// which adds curve preferences to TLSSecurityProfile for PQC readiness.
	CurvePreferences string // comma-separated curve names (e.g., "X25519,CurveP256")
}

// GetTLSEnvVarsFromAPIServer fetches the OpenShift APIServer TLS Profile
// and converts it to environment variables
func GetTLSEnvVarsFromAPIServer(ctx context.Context, restConfig *rest.Config) (*TLSEnvVars, error) {
	logger := logging.FromContext(ctx)

	// Create OpenShift config client
	configClient, err := openshiftconfigclient.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create OpenShift config client: %w", err)
	}

	// Fetch the APIServer resource
	apiServer, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			logger.Info("APIServer resource not found, skipping TLS profile injection")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get APIServer resource: %w", err)
	}

	// Get the TLS security profile
	tlsProfile := apiServer.Spec.TLSSecurityProfile
	if tlsProfile == nil {
		logger.Info("No TLS security profile configured in APIServer, skipping TLS profile injection")
		return nil, nil
	}

	logger.Infof("Found TLS security profile type: %s", tlsProfile.Type)

	// Convert the TLS profile to environment variables
	return convertTLSProfileToEnvVars(tlsProfile), nil
}

// GetTLSProfileFingerprint returns a deterministic string representing the current
// TLS security profile state from the OpenShift APIServer. This fingerprint changes when:
// - Profile type changes (Old, Intermediate, Modern, Custom)
// - For Custom profiles: minTLSVersion changes
// - For Custom profiles: cipher list changes
// Returns empty string if no profile is configured or on error.
func GetTLSProfileFingerprint(ctx context.Context, restConfig *rest.Config) string {
	logger := logging.FromContext(ctx)

	configClient, err := openshiftconfigclient.NewForConfig(restConfig)
	if err != nil {
		logger.Debugf("Failed to create OpenShift config client for TLS fingerprint: %v", err)
		return ""
	}

	apiServer, err := configClient.ConfigV1().APIServers().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			logger.Debugf("Failed to get APIServer for TLS fingerprint: %v", err)
		}
		return ""
	}

	profile := apiServer.Spec.TLSSecurityProfile
	if profile == nil {
		return ""
	}

	// For predefined profiles (Old, Intermediate, Modern), the type alone is sufficient
	// since they have fixed, well-known configurations
	if profile.Type != configv1.TLSProfileCustomType {
		return string(profile.Type)
	}

	// For Custom profiles, include type + minTLSVersion + sorted ciphers
	if profile.Custom == nil {
		return string(profile.Type)
	}

	// Sort ciphers for deterministic ordering
	ciphers := make([]string, len(profile.Custom.Ciphers))
	copy(ciphers, profile.Custom.Ciphers)
	sort.Strings(ciphers)

	// Build deterministic fingerprint: "Custom:VersionTLS12:cipher1,cipher2,..."
	return fmt.Sprintf("%s:%s:%s",
		profile.Type,
		profile.Custom.MinTLSVersion,
		strings.Join(ciphers, ","),
	)
}

// convertTLSProfileToEnvVars converts an OpenShift TLS security profile
// to environment variables using library-go's helper functions
func convertTLSProfileToEnvVars(profile *configv1.TLSSecurityProfile) *TLSEnvVars {
	// Use library-go's getSecurityProfileCiphers to extract TLS settings
	// This handles all profile types (Old, Intermediate, Modern, Custom)
	// and properly converts OpenSSL cipher names to IANA names
	minVersion, cipherSuites := getSecurityProfileCiphers(profile)

	envVars := &TLSEnvVars{
		MinVersion:   convertTLSVersionToString(minVersion),
		CipherSuites: strings.Join(cipherSuites, ","),
		// TODO(openshift/api#2583): Once the PR is merged and vendored, extract curve
		// preferences from the TLSSecurityProfile and populate CurvePreferences field.
		// This is required for PQC (Post-Quantum Cryptography) readiness.
		CurvePreferences: "",
	}

	return envVars
}

// getSecurityProfileCiphers extracts the minimum TLS version and cipher suites from TLSSecurityProfile object.
// Converts the ciphers to IANA names as supported by Go crypto/tls.
// If profile is nil, returns config defined by the Intermediate TLS Profile.
//
// TODO: Once Tekton Pipeline PR #9043 (k8s 0.32 -> 0.34 bump) is merged and propagated,
// switch to using library-go's crypto.OpenSSLToIANACipherSuites() instead of our local implementation.
// This will require upgrading to library-go v0.0.0-20251222131241-289839b3ffe8 or newer.
func getSecurityProfileCiphers(profile *configv1.TLSSecurityProfile) (configv1.TLSProtocolVersion, []string) {
	var profileType configv1.TLSProfileType
	if profile == nil {
		profileType = configv1.TLSProfileIntermediateType
	} else {
		profileType = profile.Type
	}

	var profileSpec *configv1.TLSProfileSpec
	if profileType == configv1.TLSProfileCustomType {
		if profile.Custom != nil {
			profileSpec = &profile.Custom.TLSProfileSpec
		}
	} else {
		profileSpec = configv1.TLSProfiles[profileType]
	}

	// nothing found / custom type set but no actual custom spec
	if profileSpec == nil {
		profileSpec = configv1.TLSProfiles[configv1.TLSProfileIntermediateType]
	}

	// Convert OpenSSL cipher names to IANA names using our local implementation
	// (copied from library-go to include TLS 1.3 support without dependency conflicts)
	return profileSpec.MinTLSVersion, openSSLToIANACipherSuites(profileSpec.Ciphers)
}

// convertTLSVersionToString converts OpenShift TLS version enum to string format (e.g., "1.2")
func convertTLSVersionToString(version configv1.TLSProtocolVersion) string {
	// Extract version number from enum (e.g., VersionTLS12 -> "1.2")
	switch version {
	case configv1.VersionTLS10:
		return "1.0"
	case configv1.VersionTLS11:
		return "1.1"
	case configv1.VersionTLS12:
		return "1.2"
	case configv1.VersionTLS13:
		return "1.3"
	default:
		return ""
	}
}

// openSSLToIANACipherSuites maps OpenSSL Cipher Suite names to their IANA counterparts.
// Unknown ciphers are filtered out.
//
// This function is copied from github.com/openshift/library-go/pkg/crypto to include
// TLS 1.3 cipher support without requiring dependency upgrades that conflict with
// current k8s versions (structured-merge-diff v4 vs v6).
//
// TODO: Once Tekton Pipeline PR #9043 is merged and k8s 0.34+ is adopted, replace this
// with library-go's crypto.OpenSSLToIANACipherSuites().
func openSSLToIANACipherSuites(ciphers []string) []string {
	ianaCiphers := make([]string, 0, len(ciphers))

	for _, c := range ciphers {
		if ianaCipher, found := openSSLToIANACiphersMap[c]; found {
			ianaCiphers = append(ianaCiphers, ianaCipher)
		}
	}

	return ianaCiphers
}

// TLSEnvVarNames defines the environment variable names for TLS configuration.
// Components should use these constants for consistency.
const (
	TLSMinVersionEnvVar       = "TLS_MIN_VERSION"
	TLSCipherSuitesEnvVar     = "TLS_CIPHER_SUITES"
	TLSCurvePreferencesEnvVar = "TLS_CURVE_PREFERENCES"
)

// openSSLToIANACiphersMap maps OpenSSL cipher names to IANA names.
// Copied from github.com/openshift/library-go@v0.0.0-20251222131241-289839b3ffe8/pkg/crypto/crypto.go
// to include TLS 1.3 cipher support.
var openSSLToIANACiphersMap = map[string]string{
	// TLS 1.3 ciphers
	"TLS_AES_128_GCM_SHA256":       "TLS_AES_128_GCM_SHA256",       // 0x13,0x01
	"TLS_AES_256_GCM_SHA384":       "TLS_AES_256_GCM_SHA384",       // 0x13,0x02
	"TLS_CHACHA20_POLY1305_SHA256": "TLS_CHACHA20_POLY1305_SHA256", // 0x13,0x03

	// TLS 1.2 ciphers
	"ECDHE-ECDSA-AES128-GCM-SHA256": "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",       // 0xC0,0x2B
	"ECDHE-RSA-AES128-GCM-SHA256":   "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",         // 0xC0,0x2F
	"ECDHE-ECDSA-AES256-GCM-SHA384": "TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384",       // 0xC0,0x2C
	"ECDHE-RSA-AES256-GCM-SHA384":   "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",         // 0xC0,0x30
	"ECDHE-ECDSA-CHACHA20-POLY1305": "TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256", // 0xCC,0xA9
	"ECDHE-RSA-CHACHA20-POLY1305":   "TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256",   // 0xCC,0xA8
	"ECDHE-ECDSA-AES128-SHA256":     "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256",       // 0xC0,0x23
	"ECDHE-RSA-AES128-SHA256":       "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256",         // 0xC0,0x27
	"AES128-GCM-SHA256":             "TLS_RSA_WITH_AES_128_GCM_SHA256",               // 0x00,0x9C
	"AES256-GCM-SHA384":             "TLS_RSA_WITH_AES_256_GCM_SHA384",               // 0x00,0x9D
	"AES128-SHA256":                 "TLS_RSA_WITH_AES_128_CBC_SHA256",               // 0x00,0x3C

	// TLS 1.0/1.1 ciphers
	"ECDHE-ECDSA-AES128-SHA": "TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA", // 0xC0,0x09
	"ECDHE-RSA-AES128-SHA":   "TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA",   // 0xC0,0x13
	"ECDHE-ECDSA-AES256-SHA": "TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA", // 0xC0,0x0A
	"ECDHE-RSA-AES256-SHA":   "TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA",   // 0xC0,0x14
	"AES128-SHA":             "TLS_RSA_WITH_AES_128_CBC_SHA",         // 0x00,0x2F
	"AES256-SHA":             "TLS_RSA_WITH_AES_256_CBC_SHA",         // 0x00,0x35
	"DES-CBC3-SHA":           "TLS_RSA_WITH_3DES_EDE_CBC_SHA",        // 0x00,0x0A
}
