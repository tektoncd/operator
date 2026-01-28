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
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestTLSProfileChanged(t *testing.T) {
	tests := []struct {
		name     string
		old      *configv1.APIServer
		new      *configv1.APIServer
		expected bool
	}{
		{
			name: "Both profiles nil - no change",
			old: &configv1.APIServer{
				Spec: configv1.APIServerSpec{},
			},
			new: &configv1.APIServer{
				Spec: configv1.APIServerSpec{},
			},
			expected: false,
		},
		{
			name: "Old nil, new has profile - changed",
			old: &configv1.APIServer{
				Spec: configv1.APIServerSpec{},
			},
			new: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileIntermediateType,
					},
				},
			},
			expected: true,
		},
		{
			name: "Old has profile, new nil - changed",
			old: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileIntermediateType,
					},
				},
			},
			new: &configv1.APIServer{
				Spec: configv1.APIServerSpec{},
			},
			expected: true,
		},
		{
			name: "Same profile type - no change",
			old: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileIntermediateType,
					},
				},
			},
			new: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileIntermediateType,
					},
				},
			},
			expected: false,
		},
		{
			name: "Different profile types - changed",
			old: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileIntermediateType,
					},
				},
			},
			new: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileModernType,
					},
				},
			},
			expected: true,
		},
		{
			name: "Intermediate to Old - changed",
			old: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileIntermediateType,
					},
				},
			},
			new: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileOldType,
					},
				},
			},
			expected: true,
		},
		{
			name: "Custom profile - same values - no change",
			old: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileCustomType,
						Custom: &configv1.CustomTLSProfile{
							TLSProfileSpec: configv1.TLSProfileSpec{
								Ciphers:       []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
								MinTLSVersion: configv1.VersionTLS12,
							},
						},
					},
				},
			},
			new: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileCustomType,
						Custom: &configv1.CustomTLSProfile{
							TLSProfileSpec: configv1.TLSProfileSpec{
								Ciphers:       []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
								MinTLSVersion: configv1.VersionTLS12,
							},
						},
					},
				},
			},
			expected: false,
		},
		{
			name: "Custom profile - different min version - changed",
			old: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileCustomType,
						Custom: &configv1.CustomTLSProfile{
							TLSProfileSpec: configv1.TLSProfileSpec{
								Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
								MinTLSVersion: configv1.VersionTLS12,
							},
						},
					},
				},
			},
			new: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileCustomType,
						Custom: &configv1.CustomTLSProfile{
							TLSProfileSpec: configv1.TLSProfileSpec{
								Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
								MinTLSVersion: configv1.VersionTLS13,
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Custom profile - different ciphers - changed",
			old: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileCustomType,
						Custom: &configv1.CustomTLSProfile{
							TLSProfileSpec: configv1.TLSProfileSpec{
								Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
								MinTLSVersion: configv1.VersionTLS12,
							},
						},
					},
				},
			},
			new: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileCustomType,
						Custom: &configv1.CustomTLSProfile{
							TLSProfileSpec: configv1.TLSProfileSpec{
								Ciphers:       []string{"TLS_AES_256_GCM_SHA384"},
								MinTLSVersion: configv1.VersionTLS12,
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Custom profile - more ciphers - changed",
			old: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileCustomType,
						Custom: &configv1.CustomTLSProfile{
							TLSProfileSpec: configv1.TLSProfileSpec{
								Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
								MinTLSVersion: configv1.VersionTLS12,
							},
						},
					},
				},
			},
			new: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileCustomType,
						Custom: &configv1.CustomTLSProfile{
							TLSProfileSpec: configv1.TLSProfileSpec{
								Ciphers:       []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
								MinTLSVersion: configv1.VersionTLS12,
							},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "Custom to Intermediate - changed",
			old: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileCustomType,
						Custom: &configv1.CustomTLSProfile{
							TLSProfileSpec: configv1.TLSProfileSpec{
								Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
								MinTLSVersion: configv1.VersionTLS12,
							},
						},
					},
				},
			},
			new: &configv1.APIServer{
				Spec: configv1.APIServerSpec{
					TLSSecurityProfile: &configv1.TLSSecurityProfile{
						Type: configv1.TLSProfileIntermediateType,
					},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tlsProfileChanged(tt.old, tt.new)
			if result != tt.expected {
				t.Errorf("tlsProfileChanged() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCustomProfilesEqual(t *testing.T) {
	tests := []struct {
		name     string
		old      *configv1.CustomTLSProfile
		new      *configv1.CustomTLSProfile
		expected bool
	}{
		{
			name:     "Both nil - equal",
			old:      nil,
			new:      nil,
			expected: true,
		},
		{
			name: "Old nil, new not nil - not equal",
			old:  nil,
			new: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
				},
			},
			expected: false,
		},
		{
			name: "Old not nil, new nil - not equal",
			old: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
				},
			},
			new:      nil,
			expected: false,
		},
		{
			name: "Same min version and ciphers - equal",
			old: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
					Ciphers:       []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
				},
			},
			new: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
					Ciphers:       []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
				},
			},
			expected: true,
		},
		{
			name: "Different min version - not equal",
			old: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
					Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
				},
			},
			new: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS13,
					Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
				},
			},
			expected: false,
		},
		{
			name: "Different cipher count - not equal",
			old: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
					Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
				},
			},
			new: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
					Ciphers:       []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
				},
			},
			expected: false,
		},
		{
			name: "Different cipher values - not equal",
			old: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
					Ciphers:       []string{"TLS_AES_128_GCM_SHA256"},
				},
			},
			new: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
					Ciphers:       []string{"TLS_AES_256_GCM_SHA384"},
				},
			},
			expected: false,
		},
		{
			name: "Same cipher values in different order - not equal",
			old: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
					Ciphers:       []string{"TLS_AES_128_GCM_SHA256", "TLS_AES_256_GCM_SHA384"},
				},
			},
			new: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
					Ciphers:       []string{"TLS_AES_256_GCM_SHA384", "TLS_AES_128_GCM_SHA256"},
				},
			},
			expected: false,
		},
		{
			name: "Empty cipher lists - equal",
			old: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
					Ciphers:       []string{},
				},
			},
			new: &configv1.CustomTLSProfile{
				TLSProfileSpec: configv1.TLSProfileSpec{
					MinTLSVersion: configv1.VersionTLS12,
					Ciphers:       []string{},
				},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := customProfilesEqual(tt.old, tt.new)
			if result != tt.expected {
				t.Errorf("customProfilesEqual() = %v, want %v", result, tt.expected)
			}
		})
	}
}
