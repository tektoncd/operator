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

import (
	"fmt"
	"sort"

	mf "github.com/manifestival/manifestival"
	"github.com/tektoncd/operator/pkg/apis/operator/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	apimachineryRuntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Generate builds a Manifest by merging defaults with cfg.Policies.
//
//   - Returns an empty manifest when cfg.Disabled is true.
//   - Starts with defaults keyed by name; cfg.Policies entries overwrite on
//     collision and add when new.
//   - Injects namespace into every NetworkPolicy object.
//   - Output is sorted by name for deterministic InstallerSet checksums.
func Generate(
	cfg v1alpha1.NetworkPolicyConfig,
	ns string,
	defaults []networkingv1.NetworkPolicy,
) (mf.Manifest, error) {
	if cfg.Disabled {
		return mf.Manifest{}, nil
	}

	merged := make(map[string]networkingv1.NetworkPolicy, len(defaults))
	for _, p := range defaults {
		merged[p.Name] = p
	}
	for name, spec := range cfg.Policies {
		merged[name] = networkingv1.NetworkPolicy{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       spec,
		}
	}

	names := make([]string, 0, len(merged))
	for name := range merged {
		names = append(names, name)
	}
	sort.Strings(names)

	resources := make([]unstructured.Unstructured, 0, len(names))
	for _, name := range names {
		np := merged[name]
		np.Namespace = ns
		np.TypeMeta = metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: networkingv1.SchemeGroupVersion.String(),
		}
		obj, err := apimachineryRuntime.DefaultUnstructuredConverter.ToUnstructured(&np)
		if err != nil {
			return mf.Manifest{}, fmt.Errorf("converting NetworkPolicy %q: %w", name, err)
		}
		u := unstructured.Unstructured{}
		u.SetUnstructuredContent(obj)
		resources = append(resources, u)
	}

	return mf.ManifestFrom(mf.Slice(resources))
}

// DefaultDenyPolicy returns a default-deny NetworkPolicy scoped to podSelector.
// Use an empty LabelSelector{} for a namespace-wide deny once all components support NetworkPolicy.
func DefaultDenyPolicy(name string, podSelector metav1.LabelSelector) networkingv1.NetworkPolicy {
	return networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: podSelector,
			PolicyTypes: []networkingv1.PolicyType{
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			},
		},
	}
}

// DNSEgressRule allows egress to DNS resolver pods on UDP and TCP using the
// platform-specific DNS port (53 on Kubernetes, 5353 on OpenShift).
func DNSEgressRule(p PlatformParams) networkingv1.NetworkPolicyEgressRule {
	udp := corev1.ProtocolUDP
	tcp := corev1.ProtocolTCP
	udpPort := intstr.FromInt32(p.DNSPort)
	tcpPort := intstr.FromInt32(p.DNSPort)
	return networkingv1.NetworkPolicyEgressRule{
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: &udp, Port: &udpPort},
			{Protocol: &tcp, Port: &tcpPort},
		},
		To: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{"kubernetes.io/metadata.name": p.DNSResolverNamespace},
				},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: p.DNSResolverPodLabel,
				},
			},
		},
	}
}

// APIServerEgressRule allows all egress so pods can reach the API server.
// NetworkPolicy cannot select host-network endpoints, and the API server
// port is configurable, so we must allow unrestricted egress.
func APIServerEgressRule() networkingv1.NetworkPolicyEgressRule {
	return networkingv1.NetworkPolicyEgressRule{}
}

// InternetEgressRule allows egress on TCP 80 and 443 to any destination.
func InternetEgressRule() networkingv1.NetworkPolicyEgressRule {
	tcp := corev1.ProtocolTCP
	httpPort := intstr.FromInt32(80)
	httpsPort := intstr.FromInt32(443)
	return networkingv1.NetworkPolicyEgressRule{
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: &tcp, Port: &httpPort},
			{Protocol: &tcp, Port: &httpsPort},
		},
	}
}

// SSHEgressRule allows TCP egress on port 22 for git clone over SSH.
func SSHEgressRule() networkingv1.NetworkPolicyEgressRule {
	tcp := corev1.ProtocolTCP
	sshPort := intstr.FromInt32(22)
	return networkingv1.NetworkPolicyEgressRule{
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: &tcp, Port: &sshPort},
		},
	}
}

// PrometheusIngressRule allows ingress from the monitoring namespace on the given port.
func PrometheusIngressRule(p PlatformParams, port intstr.IntOrString) networkingv1.NetworkPolicyIngressRule {
	tcp := corev1.ProtocolTCP
	return networkingv1.NetworkPolicyIngressRule{
		From: []networkingv1.NetworkPolicyPeer{
			{
				NamespaceSelector: &metav1.LabelSelector{
					MatchLabels: p.PrometheusNamespaceLabel,
				},
			},
		},
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: &tcp, Port: &port},
		},
	}
}

// WebhookIngressRule allows ingress on port for admission webhooks.
// If cidr is empty, allows from any source (no From restriction).
func WebhookIngressRule(cidr string, port intstr.IntOrString) networkingv1.NetworkPolicyIngressRule {
	tcp := corev1.ProtocolTCP
	rule := networkingv1.NetworkPolicyIngressRule{
		Ports: []networkingv1.NetworkPolicyPort{
			{Protocol: &tcp, Port: &port},
		},
	}
	if cidr != "" {
		rule.From = []networkingv1.NetworkPolicyPeer{
			{IPBlock: &networkingv1.IPBlock{CIDR: cidr}},
		}
	}
	return rule
}
