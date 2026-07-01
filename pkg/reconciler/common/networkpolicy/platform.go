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
	// DNSPort is the port the platform's DNS resolver pods listen on.
	// Vanilla Kubernetes CoreDNS pods listen on 53.
	// OpenShift DNS pods listen on 5353 (host-networked daemonset); OVN-Kubernetes
	// enforces NetworkPolicy after DNAT, so the policy must match the pod port (5353),
	// not the dns-default service port (53).
	DNSPort int32
	// APIServerPort is the port the Kubernetes API server service exposes to pods.
	// Vanilla Kubernetes exposes the api-server via the kubernetes.default.svc service on
	// port 443 (targetPort 6443). OpenShift exposes it on port 6443 directly.
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
