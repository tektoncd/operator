/*
Copyright 2024 The Tekton Authors

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

package networkpolicy

// PlatformParams holds platform-specific values for building default NetworkPolicy rules.
// It is an internal type — it never appears in CRD API fields.
type PlatformParams struct {
	DNSResolverNamespace     string
	DNSResolverPodLabel      map[string]string
	PrometheusNamespaceLabel map[string]string
	// DNSPort is the DNS resolver pod port. Kubernetes CoreDNS uses 53; OpenShift DNS
	// uses 5353 (OVN-K8s enforces NetworkPolicy after DNAT, so pod port applies).
	DNSPort int32
	// APIServerPort is the kubernetes.default.svc port: 443 on Kubernetes, 6443 on OpenShift.
	APIServerPort int32
}

// KubernetesPlatformDefaults returns PlatformParams for vanilla Kubernetes.
func KubernetesPlatformDefaults() PlatformParams {
	return PlatformParams{
		DNSResolverNamespace:     "kube-system",
		DNSResolverPodLabel:      map[string]string{"k8s-app": "kube-dns"},
		PrometheusNamespaceLabel: map[string]string{"kubernetes.io/metadata.name": "monitoring"},
		DNSPort:                  53,
		APIServerPort:            443,
	}
}

// OpenShiftPlatformDefaults returns PlatformParams for OpenShift.
func OpenShiftPlatformDefaults() PlatformParams {
	return PlatformParams{
		DNSResolverNamespace:     "openshift-dns",
		DNSResolverPodLabel:      map[string]string{"dns.operator.openshift.io/daemonset-dns": "default"},
		PrometheusNamespaceLabel: map[string]string{"openshift.io/cluster-monitoring": "true"},
		DNSPort:                  5353,
		APIServerPort:            6443,
	}
}
