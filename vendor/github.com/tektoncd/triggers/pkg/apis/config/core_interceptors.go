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

package config

import (
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	enterpriseHostAllowlistKey = "github.enterprise-host-allowlist"
)

// CoreInterceptorsConfig holds configuration for the core interceptors service.
// +k8s:deepcopy-gen=true
type CoreInterceptorsConfig struct {
	EnterpriseHostAllowlist []string
}

// GetCoreInterceptorsConfigName returns the name of the configmap containing
// core interceptor configuration.
func GetCoreInterceptorsConfigName() string {
	if e := os.Getenv("CONFIG_CORE_INTERCEPTORS_NAME"); e != "" {
		return e
	}
	return "config-triggers-core-interceptors"
}

// NewCoreInterceptorsFromMap returns a CoreInterceptors given a map corresponding to a ConfigMap
func NewCoreInterceptorsFromMap(cfgMap map[string]string) (*CoreInterceptorsConfig, error) {
	ci := CoreInterceptorsConfig{}

	if v, ok := cfgMap[enterpriseHostAllowlistKey]; ok && v != "" {
		for p := range strings.SplitSeq(v, ",") {
			if h := strings.TrimSpace(p); h != "" {
				ci.EnterpriseHostAllowlist = append(ci.EnterpriseHostAllowlist, normalizeUrl(h))
			}
		}
	}

	return &ci, nil
}

// NewCoreInterceptorsFromConfigMap returns a CoreInterceptors for the given configmap
func NewCoreInterceptorsFromConfigMap(config *corev1.ConfigMap) (*CoreInterceptorsConfig, error) {
	return NewCoreInterceptorsFromMap(config.Data)
}

// DeepCopy returns a deep copy of CoreInterceptors.
func (in *CoreInterceptorsConfig) DeepCopy() *CoreInterceptorsConfig {
	if in == nil {
		return nil
	}
	out := new(CoreInterceptorsConfig)
	if in.EnterpriseHostAllowlist != nil {
		out.EnterpriseHostAllowlist = make([]string, len(in.EnterpriseHostAllowlist))
		copy(out.EnterpriseHostAllowlist, in.EnterpriseHostAllowlist)
	}
	return out
}

func (in *CoreInterceptorsConfig) HostInEnterpriseHostAllowList(host string) bool {
	host = normalizeUrl(host)
	for _, h := range in.EnterpriseHostAllowlist {
		if strings.EqualFold(host, h) {
			return true
		}
	}
	return false
}

func normalizeUrl(url string) string {
	// Strip any scheme and path for normalization.
	// Using instead of net/url.Parse which doesn't handle missing schemes well.
	parts := strings.Split(url, "://")
	if len(parts) > 1 {
		url = parts[1]
	}
	parts = strings.Split(url, "/")
	if len(parts) > 1 {
		url = parts[0]
	}
	return url
}
