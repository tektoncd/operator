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

func TestAPIServerTLSProfileChanged_BothNil(t *testing.T) {
	old := &configv1.APIServer{}
	new := &configv1.APIServer{}
	if APIServerTLSProfileChanged(old, new) {
		t.Error("expected no change when both TLS profiles are nil")
	}
}

func TestAPIServerTLSProfileChanged_TypeChange(t *testing.T) {
	old := &configv1.APIServer{Spec: configv1.APIServerSpec{
		TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType},
	}}
	new := &configv1.APIServer{Spec: configv1.APIServerSpec{
		TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType},
	}}
	if !APIServerTLSProfileChanged(old, new) {
		t.Error("expected change when profile type differs")
	}
}

func TestAPIServerTLSProfileChanged_NilToNonNil(t *testing.T) {
	old := &configv1.APIServer{}
	new := &configv1.APIServer{Spec: configv1.APIServerSpec{
		TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileModernType},
	}}
	if !APIServerTLSProfileChanged(old, new) {
		t.Error("expected change when old is nil and new is non-nil")
	}
}

func TestAPIServerTLSProfileChanged_SameType(t *testing.T) {
	old := &configv1.APIServer{Spec: configv1.APIServerSpec{
		TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType},
	}}
	new := &configv1.APIServer{Spec: configv1.APIServerSpec{
		TLSSecurityProfile: &configv1.TLSSecurityProfile{Type: configv1.TLSProfileIntermediateType},
	}}
	if APIServerTLSProfileChanged(old, new) {
		t.Error("expected no change for same named profile type")
	}
}

func TestAPIServerTLSProfileChanged_CustomCipherChange(t *testing.T) {
	customOld := &configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileCustomType,
		Custom: &configv1.CustomTLSProfile{TLSProfileSpec: configv1.TLSProfileSpec{
			MinTLSVersion: configv1.VersionTLS12,
			Ciphers:       []string{"TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256"},
		}},
	}
	customNew := &configv1.TLSSecurityProfile{
		Type: configv1.TLSProfileCustomType,
		Custom: &configv1.CustomTLSProfile{TLSProfileSpec: configv1.TLSProfileSpec{
			MinTLSVersion: configv1.VersionTLS12,
			Ciphers:       []string{"TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384"},
		}},
	}
	old := &configv1.APIServer{Spec: configv1.APIServerSpec{TLSSecurityProfile: customOld}}
	new := &configv1.APIServer{Spec: configv1.APIServerSpec{TLSSecurityProfile: customNew}}
	if !APIServerTLSProfileChanged(old, new) {
		t.Error("expected change when custom cipher differs")
	}
}
