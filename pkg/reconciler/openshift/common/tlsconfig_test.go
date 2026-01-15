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
	"crypto/tls"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestTLSVersionToString(t *testing.T) {
	tests := []struct {
		name    string
		version uint16
		want    string
	}{
		{
			name:    "TLS 1.0",
			version: tls.VersionTLS10,
			want:    "TLSv1.0",
		},
		{
			name:    "TLS 1.1",
			version: tls.VersionTLS11,
			want:    "TLSv1.1",
		},
		{
			name:    "TLS 1.2",
			version: tls.VersionTLS12,
			want:    "TLSv1.2",
		},
		{
			name:    "TLS 1.3",
			version: tls.VersionTLS13,
			want:    "TLSv1.3",
		},
		{
			name:    "Unknown version defaults to TLS 1.2",
			version: 0,
			want:    "TLSv1.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TLSVersionToString(tt.version)
			if got != tt.want {
				t.Errorf("TLSVersionToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCipherSuitesToNames(t *testing.T) {
	tests := []struct {
		name   string
		suites []uint16
		want   []string
	}{
		{
			name:   "Empty slice",
			suites: []uint16{},
			want:   []string{},
		},
		{
			name: "TLS 1.3 ciphers",
			suites: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
				tls.TLS_CHACHA20_POLY1305_SHA256,
			},
			want: []string{
				"TLS_AES_128_GCM_SHA256",
				"TLS_AES_256_GCM_SHA384",
				"TLS_CHACHA20_POLY1305_SHA256",
			},
		},
		{
			name: "TLS 1.2 ciphers",
			suites: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
			want: []string{
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CipherSuitesToNames(tt.suites)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("CipherSuitesToNames() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCurveIDsToString(t *testing.T) {
	tests := []struct {
		name   string
		curves []tls.CurveID
		want   string
	}{
		{
			name:   "Empty slice",
			curves: []tls.CurveID{},
			want:   "",
		},
		{
			name: "FIPS P-curves",
			curves: []tls.CurveID{
				tls.CurveP256,
				tls.CurveP384,
				tls.CurveP521,
			},
			want: "P-256,P-384,P-521",
		},
		{
			name: "X25519 curve",
			curves: []tls.CurveID{
				tls.X25519,
			},
			want: "X25519",
		},
		{
			name: "Mixed curves",
			curves: []tls.CurveID{
				tls.X25519,
				tls.CurveP256,
				tls.CurveP384,
			},
			want: "X25519,P-256,P-384",
		},
		{
			name: "Unknown curve",
			curves: []tls.CurveID{
				tls.CurveID(999),
			},
			want: "curve-999",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CurveIDsToString(tt.curves)
			if got != tt.want {
				t.Errorf("CurveIDsToString() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCalculateTLSConfigHash(t *testing.T) {
	tests := []struct {
		name   string
		config *tls.Config
		want   string
	}{
		{
			name:   "Nil config returns empty string",
			config: nil,
			want:   "",
		},
		{
			name: "Same config produces same hash",
			config: &tls.Config{
				MinVersion: tls.VersionTLS13,
				CipherSuites: []uint16{
					tls.TLS_AES_128_GCM_SHA256,
					tls.TLS_AES_256_GCM_SHA384,
				},
				CurvePreferences: []tls.CurveID{
					tls.CurveP256,
					tls.CurveP384,
					tls.CurveP521,
				},
			},
			want: "", // We'll check it's not empty and consistent
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateTLSConfigHash(tt.config)
			if tt.config == nil {
				if got != tt.want {
					t.Errorf("CalculateTLSConfigHash(nil) = %v, want %v", got, tt.want)
				}
			} else {
				// For non-nil configs, verify hash is deterministic
				got2 := CalculateTLSConfigHash(tt.config)
				if got != got2 {
					t.Errorf("CalculateTLSConfigHash() not deterministic: %v != %v", got, got2)
				}
				if len(got) != 16 {
					t.Errorf("CalculateTLSConfigHash() hash length = %v, want 16", len(got))
				}
			}
		})
	}

	// Test that different configs produce different hashes
	t.Run("Different configs produce different hashes", func(t *testing.T) {
		config1 := &tls.Config{MinVersion: tls.VersionTLS12}
		config2 := &tls.Config{MinVersion: tls.VersionTLS13}

		hash1 := CalculateTLSConfigHash(config1)
		hash2 := CalculateTLSConfigHash(config2)

		if hash1 == hash2 {
			t.Errorf("Different configs produced same hash: %v", hash1)
		}
	})
}

func TestTLSEnvVarsFromConfig(t *testing.T) {
	tests := []struct {
		name   string
		config *tls.Config
		want   *TLSEnvVars
	}{
		{
			name:   "Nil config returns nil",
			config: nil,
			want:   nil,
		},
		{
			name: "TLS 1.3 with TLS 1.3 ciphers",
			config: &tls.Config{
				MinVersion: tls.VersionTLS13,
				CipherSuites: []uint16{
					tls.TLS_AES_128_GCM_SHA256,
					tls.TLS_AES_256_GCM_SHA384,
					tls.TLS_CHACHA20_POLY1305_SHA256,
				},
				CurvePreferences: []tls.CurveID{
					tls.CurveP256,
					tls.CurveP384,
					tls.CurveP521,
				},
			},
			want: &TLSEnvVars{
				MinVersion:       "TLSv1.3",
				CipherSuites:     "TLS_AES_128_GCM_SHA256,TLS_AES_256_GCM_SHA384,TLS_CHACHA20_POLY1305_SHA256",
				CurvePreferences: "P-256,P-384,P-521",
			},
		},
		{
			name: "TLS 1.2 with TLS 1.2 ciphers",
			config: &tls.Config{
				MinVersion: tls.VersionTLS12,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				},
				CurvePreferences: []tls.CurveID{
					tls.CurveP256,
					tls.CurveP384,
				},
			},
			want: &TLSEnvVars{
				MinVersion:       "TLSv1.2",
				CipherSuites:     "TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
				CurvePreferences: "P-256,P-384",
			},
		},
		{
			name: "Empty cipher suites and curves",
			config: &tls.Config{
				MinVersion: tls.VersionTLS13,
			},
			want: &TLSEnvVars{
				MinVersion:       "TLSv1.3",
				CipherSuites:     "",
				CurvePreferences: "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TLSEnvVarsFromConfig(tt.config)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("TLSEnvVarsFromConfig() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestGetTLSConfigFromContext(t *testing.T) {
	tests := []struct {
		name  string
		setup func() context.Context
		want  *tls.Config
	}{
		{
			name: "Context without TLS config returns nil",
			setup: func() context.Context {
				return context.Background()
			},
			want: nil,
		},
		{
			name: "Context with TLS config returns config",
			setup: func() context.Context {
				config := &tls.Config{
					MinVersion: tls.VersionTLS13,
				}
				ctx := context.Background()
				ctx = context.WithValue(ctx, tlsConfigKey{}, config)
				return ctx
			},
			want: &tls.Config{
				MinVersion: tls.VersionTLS13,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			got := GetTLSConfigFromContext(ctx)
			if tt.want == nil {
				if got != nil {
					t.Errorf("GetTLSConfigFromContext() = %v, want nil", got)
				}
			} else {
				if got == nil {
					t.Errorf("GetTLSConfigFromContext() = nil, want non-nil")
				} else if got.MinVersion != tt.want.MinVersion {
					t.Errorf("GetTLSConfigFromContext().MinVersion = %v, want %v", got.MinVersion, tt.want.MinVersion)
				}
			}
		})
	}
}

func TestGetTLSConfigHashFromContext(t *testing.T) {
	tests := []struct {
		name  string
		setup func() context.Context
		want  string
	}{
		{
			name: "Context without hash returns empty string",
			setup: func() context.Context {
				return context.Background()
			},
			want: "",
		},
		{
			name: "Context with hash returns hash",
			setup: func() context.Context {
				ctx := context.Background()
				ctx = context.WithValue(ctx, tlsConfigHashKey{}, "abc123def456")
				return ctx
			},
			want: "abc123def456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			got := GetTLSConfigHashFromContext(ctx)
			if got != tt.want {
				t.Errorf("GetTLSConfigHashFromContext() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetTLSEnvVarsFromContext(t *testing.T) {
	tests := []struct {
		name  string
		setup func() context.Context
		want  *TLSEnvVars
	}{
		{
			name: "Context without TLS config returns nil",
			setup: func() context.Context {
				return context.Background()
			},
			want: nil,
		},
		{
			name: "Context with TLS config returns env vars",
			setup: func() context.Context {
				config := &tls.Config{
					MinVersion: tls.VersionTLS13,
					CipherSuites: []uint16{
						tls.TLS_AES_128_GCM_SHA256,
					},
					CurvePreferences: []tls.CurveID{
						tls.CurveP256,
					},
				}
				ctx := context.Background()
				ctx = context.WithValue(ctx, tlsConfigKey{}, config)
				return ctx
			},
			want: &TLSEnvVars{
				MinVersion:       "TLSv1.3",
				CipherSuites:     "TLS_AES_128_GCM_SHA256",
				CurvePreferences: "P-256",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setup()
			got := GetTLSEnvVarsFromContext(ctx)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("GetTLSEnvVarsFromContext() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestIsCentralTLSConfigEnabled(t *testing.T) {
	tests := []struct {
		name   string
		envVal string
		want   bool
	}{
		{
			name:   "Empty env var defaults to enabled",
			envVal: "",
			want:   true,
		},
		{
			name:   "Explicitly enabled with 'true'",
			envVal: "true",
			want:   true,
		},
		{
			name:   "Explicitly enabled with '1'",
			envVal: "1",
			want:   true,
		},
		{
			name:   "Explicitly disabled with 'false'",
			envVal: "false",
			want:   false,
		},
		{
			name:   "Explicitly disabled with '0'",
			envVal: "0",
			want:   false,
		},
		{
			name:   "Invalid value defaults to enabled",
			envVal: "invalid",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save and restore original env var
			oldVal := os.Getenv(EnableCentralTLSConfigEnvVar)
			defer func() {
				if oldVal == "" {
					os.Unsetenv(EnableCentralTLSConfigEnvVar)
				} else {
					os.Setenv(EnableCentralTLSConfigEnvVar, oldVal)
				}
			}()

			// Set test env var
			if tt.envVal == "" {
				os.Unsetenv(EnableCentralTLSConfigEnvVar)
			} else {
				os.Setenv(EnableCentralTLSConfigEnvVar, tt.envVal)
			}

			got := IsCentralTLSConfigEnabled()
			if got != tt.want {
				t.Errorf("IsCentralTLSConfigEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCipherSuitesFromIANANames(t *testing.T) {
	tests := []struct {
		name  string
		names []string
		want  []uint16
	}{
		{
			name:  "Empty names returns empty slice",
			names: []string{},
			want:  []uint16{},
		},
		{
			name: "Valid TLS 1.3 cipher names",
			names: []string{
				"TLS_AES_128_GCM_SHA256",
				"TLS_AES_256_GCM_SHA384",
			},
			want: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
			},
		},
		{
			name: "Valid TLS 1.2 cipher names",
			names: []string{
				"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256",
				"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384",
			},
			want: []uint16{
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			},
		},
		{
			name: "Invalid cipher names are filtered out",
			names: []string{
				"TLS_AES_128_GCM_SHA256",
				"INVALID_CIPHER",
				"TLS_AES_256_GCM_SHA384",
			},
			want: []uint16{
				tls.TLS_AES_128_GCM_SHA256,
				tls.TLS_AES_256_GCM_SHA384,
			},
		},
		{
			name: "All invalid cipher names returns empty slice",
			names: []string{
				"INVALID_CIPHER_1",
				"INVALID_CIPHER_2",
			},
			want: []uint16{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cipherSuitesFromIANANames(tt.names)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("cipherSuitesFromIANANames() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestParseTLSVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    uint16
		wantErr bool
	}{
		{
			name:    "VersionTLS10",
			version: "VersionTLS10",
			want:    tls.VersionTLS10,
			wantErr: false,
		},
		{
			name:    "TLSv1.0",
			version: "TLSv1.0",
			want:    tls.VersionTLS10,
			wantErr: false,
		},
		{
			name:    "VersionTLS11",
			version: "VersionTLS11",
			want:    tls.VersionTLS11,
			wantErr: false,
		},
		{
			name:    "TLSv1.1",
			version: "TLSv1.1",
			want:    tls.VersionTLS11,
			wantErr: false,
		},
		{
			name:    "VersionTLS12",
			version: "VersionTLS12",
			want:    tls.VersionTLS12,
			wantErr: false,
		},
		{
			name:    "TLSv1.2",
			version: "TLSv1.2",
			want:    tls.VersionTLS12,
			wantErr: false,
		},
		{
			name:    "VersionTLS13",
			version: "VersionTLS13",
			want:    tls.VersionTLS13,
			wantErr: false,
		},
		{
			name:    "TLSv1.3",
			version: "TLSv1.3",
			want:    tls.VersionTLS13,
			wantErr: false,
		},
		{
			name:    "Unknown version returns error",
			version: "UnknownVersion",
			want:    0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTLSVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTLSVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("parseTLSVersion() = %v, want %v", got, tt.want)
			}
		})
	}
}
