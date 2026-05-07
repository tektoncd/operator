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

package tektonconfig

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
	occommon "github.com/tektoncd/operator/pkg/reconciler/openshift/common"
	"gotest.tools/v3/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestTlsProfileChanged(t *testing.T) {
	tests := []struct {
		name     string
		old      *configv1.TLSSecurityProfile
		new      *configv1.TLSSecurityProfile
		expected bool
	}{
		{
			name:     "both nil",
			old:      nil,
			new:      nil,
			expected: false,
		},
		{
			name:     "old nil, new set",
			old:      nil,
			new:      &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType},
			expected: true,
		},
		{
			name:     "old set, new nil",
			old:      &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType},
			new:      nil,
			expected: true,
		},
		{
			name:     "same predefined type",
			old:      &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType},
			new:      &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType},
			expected: false,
		},
		{
			name:     "different predefined types",
			old:      &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType},
			new:      &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType},
			expected: true,
		},
		{
			name: "predefined to custom",
			old:  &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType},
			new: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
					},
				},
			},
			expected: true,
		},
		{
			name: "same custom profiles",
			old: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
						Ciphers:       []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
				},
			},
			new: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
						Ciphers:       []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
				},
			},
			expected: false,
		},
		{
			name: "custom profiles different min version",
			old: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
					},
				},
			},
			new: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS13,
					},
				},
			},
			expected: true,
		},
		{
			name: "custom profiles different ciphers",
			old: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
						Ciphers:       []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
				},
			},
			new: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
						Ciphers:       []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
					},
				},
			},
			expected: true,
		},
		{
			name: "custom profiles different cipher count",
			old: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						Ciphers: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
					},
				},
			},
			new: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						Ciphers: []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256", "TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
					},
				},
			},
			expected: true,
		},
		{
			name: "custom type with one nil Custom field",
			old: &configv1.TLSSecurityProfile{
				Type:   configv1.TLSProfileCustomType,
				Custom: nil,
			},
			new: &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
					},
				},
			},
			expected: true,
		},
		{
			name: "custom type with both nil Custom fields",
			old: &configv1.TLSSecurityProfile{
				Type:   configv1.TLSProfileCustomType,
				Custom: nil,
			},
			new: &configv1.TLSSecurityProfile{
				Type:   configv1.TLSProfileCustomType,
				Custom: nil,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := &configv1.APIServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       configv1.APIServerSpec{TLSSecurityProfile: tt.old},
			}
			new := &configv1.APIServer{
				ObjectMeta: metav1.ObjectMeta{Name: "cluster"},
				Spec:       configv1.APIServerSpec{TLSSecurityProfile: tt.new},
			}
			assert.Equal(t, occommon.APIServerTLSProfileChanged(old, new), tt.expected)
		})
	}
}
