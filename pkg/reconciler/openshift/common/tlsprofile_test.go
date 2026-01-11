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
	"strings"
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestConvertTLSVersionToString(t *testing.T) {
	tests := []struct {
		name     string
		version  configv1.TLSProtocolVersion
		expected string
	}{
		{
			name:     "TLS 1.0",
			version:  configv1.VersionTLS10,
			expected: "1.0",
		},
		{
			name:     "TLS 1.1",
			version:  configv1.VersionTLS11,
			expected: "1.1",
		},
		{
			name:     "TLS 1.2",
			version:  configv1.VersionTLS12,
			expected: "1.2",
		},
		{
			name:     "TLS 1.3",
			version:  configv1.VersionTLS13,
			expected: "1.3",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertTLSVersionToString(tt.version)
			if result != tt.expected {
				t.Errorf("convertTLSVersionToString(%v) = %v, want %v", tt.version, result, tt.expected)
			}
		})
	}
}

func TestGetSecurityProfileCiphers(t *testing.T) {
	tests := []struct {
		name               string
		profile            *configv1.TLSSecurityProfile
		expectedMinVersion configv1.TLSProtocolVersion
		minCipherCount     int // Minimum expected ciphers
	}{
		{
			name:               "Nil profile defaults to Intermediate",
			profile:            nil,
			expectedMinVersion: configv1.VersionTLS12,
			minCipherCount:     5, // At least 5 ciphers should be converted
		},
		{
			name: "Intermediate profile",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileIntermediateType,
			},
			expectedMinVersion: configv1.VersionTLS12,
			minCipherCount:     5,
		},
		{
			name: "Modern profile (TLS 1.3)",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileModernType,
			},
			expectedMinVersion: configv1.VersionTLS13,
			minCipherCount:     3, // TLS 1.3 has 3 standard ciphers
		},
		{
			name: "Custom profile with OpenSSL names",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
						Ciphers:       []string{"ECDHE-ECDSA-AES128-GCM-SHA256", "ECDHE-RSA-AES128-GCM-SHA256"},
					},
				},
			},
			expectedMinVersion: configv1.VersionTLS12,
			minCipherCount:     2, // Both should be converted
		},
		{
			name: "Custom profile with TLS 1.3 ciphers",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS13,
						Ciphers:       []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
					},
				},
			},
			expectedMinVersion: configv1.VersionTLS13,
			minCipherCount:     2, // TLS 1.3 ciphers now supported
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			minVersion, cipherSuites := getSecurityProfileCiphers(tt.profile)
			if minVersion != tt.expectedMinVersion {
				t.Errorf("minVersion = %v, want %v", minVersion, tt.expectedMinVersion)
			}
			if len(cipherSuites) < tt.minCipherCount {
				t.Errorf("cipher count = %d, want at least %d", len(cipherSuites), tt.minCipherCount)
			}
			// Verify ciphers are IANA names (should contain underscores)
			for _, cipher := range cipherSuites {
				if cipher != "" && !strings.Contains(cipher, "TLS_") {
					t.Errorf("cipher %s doesn't look like IANA format", cipher)
				}
			}
		})
	}
}

func TestOpenSSLToIANAConversion(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name: "TLS 1.2 OpenSSL ciphers",
			input: []string{
				"ECDHE-ECDSA-AES128-GCM-SHA256",
				"ECDHE-RSA-AES128-GCM-SHA256",
			},
			expected: []string{
				"TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			},
		},
		{
			name: "TLS 1.3 ciphers (already IANA format)",
			input: []string{
				"TLS_AES_128_GCM_SHA256",
				"TLS_AES_256_GCM_SHA384",
				"TLS_CHACHA20_POLY1305_SHA256",
			},
			expected: []string{
				"TLS_AES_128_GCM_SHA256",
				"TLS_AES_256_GCM_SHA384",
				"TLS_CHACHA20_POLY1305_SHA256",
			},
		},
		{
			name: "Mixed TLS 1.2 and 1.3 ciphers",
			input: []string{
				"TLS_AES_128_GCM_SHA256",
				"ECDHE-RSA-AES128-GCM-SHA256",
			},
			expected: []string{
				"TLS_AES_128_GCM_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			},
		},
		{
			name:     "Unknown cipher filtered out",
			input:    []string{"UNKNOWN-CIPHER", "ECDHE-RSA-AES128-GCM-SHA256"},
			expected: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := openSSLToIANACipherSuites(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d ciphers, got %d", len(tt.expected), len(result))
				return
			}
			for i, cipher := range result {
				if cipher != tt.expected[i] {
					t.Errorf("cipher[%d] = %s, want %s", i, cipher, tt.expected[i])
				}
			}
		})
	}
}

func TestConvertTLSProfileToEnvVars(t *testing.T) {
	tests := []struct {
		name     string
		profile  *configv1.TLSSecurityProfile
		expected *TLSEnvVars
	}{
		{
			name: "Intermediate profile",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileIntermediateType,
			},
			expected: &TLSEnvVars{
				MinVersion: "1.2",
				// CipherSuites will be the converted intermediate ciphers
				CipherSuites: strings.Join(openSSLToIANACipherSuites(configv1.TLSProfiles[configv1.TLSProfileIntermediateType].Ciphers), ","),
			},
		},
		{
			name: "Modern profile",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileModernType,
			},
			expected: &TLSEnvVars{
				MinVersion: "1.3",
				// CipherSuites will be the converted modern ciphers (TLS 1.3)
				CipherSuites: strings.Join(openSSLToIANACipherSuites(configv1.TLSProfiles[configv1.TLSProfileModernType].Ciphers), ","),
			},
		},
		{
			name: "Custom profile with OpenSSL cipher names",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
						Ciphers:       []string{"ECDHE-ECDSA-AES128-GCM-SHA256", "ECDHE-RSA-AES128-GCM-SHA256"},
					},
				},
			},
			expected: &TLSEnvVars{
				MinVersion:   "1.2",
				CipherSuites: "TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertTLSProfileToEnvVars(tt.profile)
			if result.MinVersion != tt.expected.MinVersion {
				t.Errorf("MinVersion = %v, want %v", result.MinVersion, tt.expected.MinVersion)
			}
			if result.CipherSuites != tt.expected.CipherSuites {
				t.Errorf("CipherSuites = %v, want %v", result.CipherSuites, tt.expected.CipherSuites)
			}
		})
	}
}
