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
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestConvertTLSVersionToEnvFormat(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		expected  string
		expectErr bool
	}{
		{"TLS 1.0", "VersionTLS10", "1.0", false},
		{"TLS 1.1", "VersionTLS11", "1.1", false},
		{"TLS 1.2", "VersionTLS12", "1.2", false},
		{"TLS 1.3", "VersionTLS13", "1.3", false},
		{"Unknown version", "UnknownVersion", "", true},
		{"Empty string", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertTLSVersionToEnvFormat(tt.version)
			if tt.expectErr && err == nil {
				t.Errorf("convertTLSVersionToEnvFormat(%s) expected error, got nil", tt.version)
			}
			if !tt.expectErr && err != nil {
				t.Errorf("convertTLSVersionToEnvFormat(%s) unexpected error: %v", tt.version, err)
			}
			if result != tt.expected {
				t.Errorf("convertTLSVersionToEnvFormat(%s) = %s, want %s", tt.version, result, tt.expected)
			}
		})
	}
}

func TestSupplementTLS13Ciphers(t *testing.T) {
	tests := []struct {
		name            string
		profile         *configv1.TLSSecurityProfile
		observedCiphers []string
		expectContains  []string
	}{
		{
			name:            "Nil profile returns observed ciphers unchanged",
			profile:         nil,
			observedCiphers: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
			expectContains:  []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
		},
		{
			name: "Custom profile with TLS 1.3 ciphers supplements missing ones",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						Ciphers: []string{
							"TLS_AES_128_GCM_SHA256",
							"TLS_AES_256_GCM_SHA384",
						},
					},
				},
			},
			observedCiphers: []string{},
			expectContains:  []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
		},
		{
			name: "Mixed ciphers - TLS 1.3 supplemented, TLS 1.2 kept",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						Ciphers: []string{
							"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
							"TLS_AES_128_GCM_SHA256",
						},
					},
				},
			},
			observedCiphers: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
			expectContains:  []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_AES_128_GCM_SHA256"},
		},
		{
			name: "Already present TLS 1.3 ciphers not duplicated",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						Ciphers: []string{
							"TLS_AES_128_GCM_SHA256",
						},
					},
				},
			},
			observedCiphers: []string{"TLS_AES_128_GCM_SHA256"},
			expectContains:  []string{"TLS_AES_128_GCM_SHA256"},
		},
		{
			name: "Modern profile type uses predefined profile spec",
			profile: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileModernType,
			},
			observedCiphers: []string{},
			// Modern profile includes TLS 1.3 ciphers in predefined spec
			expectContains: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := supplementTLS13Ciphers(tt.profile, tt.observedCiphers)

			for _, expected := range tt.expectContains {
				found := false
				for _, cipher := range result {
					if cipher == expected {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected cipher %s not found in result %v", expected, result)
				}
			}
		})
	}
}

func TestTLSEnvVarsFromProfile(t *testing.T) {
	t.Run("nil config returns nil", func(t *testing.T) {
		result, err := TLSEnvVarsFromProfile(nil)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("Expected nil, got %v", result)
		}
	})

	t.Run("valid TLS 1.2 profile", func(t *testing.T) {
		cfg := &TLSProfileConfig{
			MinTLSVersion: "VersionTLS12",
			CipherSuites:  []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_AES_128_GCM_SHA256"},
		}
		result, err := TLSEnvVarsFromProfile(cfg)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result.MinVersion != "1.2" {
			t.Errorf("MinVersion = %s, want 1.2", result.MinVersion)
		}
		if result.CipherSuites != "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_AES_128_GCM_SHA256" {
			t.Errorf("CipherSuites = %s, want TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_AES_128_GCM_SHA256", result.CipherSuites)
		}
	})

	t.Run("valid TLS 1.3 profile", func(t *testing.T) {
		cfg := &TLSProfileConfig{
			MinTLSVersion: "VersionTLS13",
			CipherSuites:  []string{"TLS_AES_128_GCM_SHA256"},
		}
		result, err := TLSEnvVarsFromProfile(cfg)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result.MinVersion != "1.3" {
			t.Errorf("MinVersion = %s, want 1.3", result.MinVersion)
		}
	})

	t.Run("invalid TLS version rejects entire config", func(t *testing.T) {
		cfg := &TLSProfileConfig{
			MinTLSVersion: "InvalidVersion",
			CipherSuites:  []string{"TLS_AES_128_GCM_SHA256"},
		}
		result, err := TLSEnvVarsFromProfile(cfg)
		if err == nil {
			t.Error("Expected error for invalid TLS version, got nil")
		}
		if result != nil {
			t.Errorf("Expected nil result on error, got %v", result)
		}
	})

	t.Run("empty cipher suites", func(t *testing.T) {
		cfg := &TLSProfileConfig{
			MinTLSVersion: "VersionTLS12",
			CipherSuites:  nil,
		}
		result, err := TLSEnvVarsFromProfile(cfg)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if result.CipherSuites != "" {
			t.Errorf("CipherSuites = %s, want empty", result.CipherSuites)
		}
	})
}

func TestAPIServerListersInterface(t *testing.T) {
	// Verify that APIServerListers implements the interface methods correctly
	listers := &APIServerListers{}

	// These should not panic
	_ = listers.ResourceSyncer()
	_ = listers.PreRunHasSynced()
	_ = listers.APIServerLister()
}
