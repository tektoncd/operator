/*
Copyright 2022 The Tekton Authors

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

const (
	HubDbSecretName  = "tekton-hub-db"
	HubApiSecretName = "tekton-hub-api"
)

var (
	_ TektonComponent     = (*TektonHub)(nil)
	_ TektonComponentSpec = (*TektonHubSpec)(nil)
)

// TektonHub is the Schema for the tektonhub API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonHub struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonHubSpec   `json:"spec,omitempty"`
	Status TektonHubStatus `json:"status,omitempty"`
}

type TektonHubSpec struct {
	CommonSpec `json:",inline"`
	Hub        `json:",inline"`
	Categories []string       `json:"categories,omitempty"`
	Catalogs   []Catalog      `json:"catalogs,omitempty"`
	Scopes     []Scope        `json:"scopes,omitempty"`
	Default    Default        `json:"default,omitempty"`
	Db         DbSpec         `json:"db,omitempty"`
	Api        ApiSpec        `json:"api,omitempty"`
	CustomLogo CustomLogoSpec `json:"customLogo,omitempty"`
}

// Hub defines the field to customize Hub component
type Hub struct {
	// Params is the list of params passed for Hub customization
	// +optional
	Params []Param `json:"params,omitempty"`
	// options holds additions fields and these fields will be updated on the manifests
	Options AdditionalOptions `json:"options"`
}

type DbSpec struct {
	DbSecretName string `json:"secret,omitempty"`
}

type ApiSpec struct {
	// Deprecated, will be removed in further release
	HubConfigUrl           string `json:"hubConfigUrl,omitempty"`
	ApiSecretName          string `json:"secret,omitempty"`
	RouteHostUrl           string `json:"routeHostUrl,omitempty"`
	CatalogRefreshInterval string `json:"catalogRefreshInterval,omitempty"`
}

type Category struct {
	Name string `json:"name,omitempty"`
}

type Catalog struct {
	Name       string `json:"name,omitempty"`
	Org        string `json:"org,omitempty"`
	Type       string `json:"type,omitempty"`
	URL        string `json:"url,omitempty"`
	SshUrl     string `json:"sshUrl,omitempty"`
	ContextDir string `json:"contextDir,omitempty"`
	Revision   string `json:"revision,omitempty"`
	Provider   string `json:"provider,omitempty"`
}

type Scope struct {
	Name  string   `json:"name,omitempty"`
	Users []string `json:"users,omitempty"`
}

type Default struct {
	Scopes []string `json:"scopes,omitempty"`
}

// The Base64 Encode data and mediaType of the Custom Logo
type CustomLogoSpec struct {
	Base64Data string `json:"base64Data,omitempty"`
	MediaType  string `json:"mediaType,omitempty"`
}

// TektonHubStatus defines the observed state of TektonHub
type TektonHubStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The url links of the manifests, separated by comma
	// +optional
	Manifests []string `json:"manifests,omitempty"`

	// The URL route for API which needs to be exposed
	// +optional
	ApiRouteUrl string `json:"apiUrl,omitempty"`

	// The URL route for Auth server
	// +optional
	AuthRouteUrl string `json:"authUrl,omitempty"`

	// The URL route for UI which needs to be exposed
	// +optional
	UiRouteUrl string `json:"uiUrl,omitempty"`

	// The current installer set name
	// +optional
	HubInstallerSet map[string]string `json:"hubInstallerSets,omitempty"`
}

func (in *TektonHubStatus) MarkInstallerSetReady() {
	//TODO implement me
	panic("implement me")
}

func (in *TektonHubStatus) MarkInstallerSetNotReady(s string) {
	//TODO implement me
	panic("implement me")
}

func (in *TektonHubStatus) MarkInstallerSetAvailable() {
	//TODO implement me
	panic("implement me")
}

// TektonHubList contains a list of TektonHub
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonHubList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonHub `json:"items"`
}

// GetSpec implements TektonComponent
func (th *TektonHub) GetSpec() TektonComponentSpec {
	return &th.Spec
}

// GetStatus implements TektonComponent
func (th *TektonHub) GetStatus() TektonComponentStatus {
	return &th.Status
}

func (h Hub) IsEmpty() bool {
	return len(h.Params) == 0
}
