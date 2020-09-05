/*
Copyright 2019 The Knative Authors

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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	_ KComponent     = (*KnativeServing)(nil)
	_ KComponentSpec = (*KnativeServingSpec)(nil)
)

// KnativeServing is the Schema for the knativeservings API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KnativeServing struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KnativeServingSpec   `json:"spec,omitempty"`
	Status KnativeServingStatus `json:"status,omitempty"`
}

// GetSpec implements KComponent
func (ks *KnativeServing) GetSpec() KComponentSpec {
	return &ks.Spec
}

// GetStatus implements KComponent
func (ks *KnativeServing) GetStatus() KComponentStatus {
	return &ks.Status
}

// KnativeServingSpec defines the desired state of KnativeServing
type KnativeServingSpec struct {
	CommonSpec `json:",inline"`

	// A means to override the knative-ingress-gateway
	KnativeIngressGateway IstioGatewayOverride `json:"knative-ingress-gateway,omitempty"`

	// A means to override the cluster-local-gateway
	ClusterLocalGateway IstioGatewayOverride `json:"cluster-local-gateway,omitempty"`

	// Enables controller to trust registries with self-signed certificates
	ControllerCustomCerts CustomCerts `json:"controller-custom-certs,omitempty"`

	// Allows specification of HA control plane
	// +optional
	HighAvailability *HighAvailability `json:"high-availability,omitempty"`
}

// KnativeServingStatus defines the observed state of KnativeServing
type KnativeServingStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The url links of the manifests, separated by comma
	// +optional
	Manifests []string `json:"manifests,omitempty"`
}

// KnativeServingList contains a list of KnativeServing
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KnativeServingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KnativeServing `json:"items"`
}

// IstioGatewayOverride override the knative-ingress-gateway and cluster-local-gateway
type IstioGatewayOverride struct {
	// A map of values to replace the "selector" values in the knative-ingress-gateway and cluster-local-gateway
	Selector map[string]string `json:"selector,omitempty"`
}

// CustomCerts refers to either a ConfigMap or Secret containing valid
// CA certificates
type CustomCerts struct {
	// One of ConfigMap or Secret
	Type string `json:"type"`

	// The name of the ConfigMap or Secret
	Name string `json:"name"`
}

// HighAvailability specifies options for deploying Knative Serving control
// plane in a highly available manner. Note that HighAvailability is still in
// progress and does not currently provide a completely HA control plane.
type HighAvailability struct {
	// Replicas is the number of replicas that HA parts of the control plane
	// will be scaled to.
	Replicas int32 `json:"replicas"`
}
