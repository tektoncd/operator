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
	"encoding/json"
	"fmt"
	"strconv"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	_ TektonComponent     = (*TektonChain)(nil)
	_ TektonComponentSpec = (*TektonChainSpec)(nil)
)

// TektonChain is the Schema for the tektonchain API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonChain struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonChainSpec   `json:"spec,omitempty"`
	Status TektonChainStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (tc *TektonChain) GetSpec() TektonComponentSpec {
	return &tc.Spec
}

// GetStatus implements TektonComponent
func (tc *TektonChain) GetStatus() TektonComponentStatus {
	return &tc.Status
}

// TektonChainSpec defines the desired state of TektonChain
type TektonChainSpec struct {
	CommonSpec `json:",inline"`
	Chain      `json:",inline"`
	// Config holds the configuration for resources created by TektonChain
	// +optional
	Config Config `json:"config,omitempty"`
}

// Chain defines the field to provide chain configuration
type Chain struct {
	// taskrun artifacts config
	ArtifactsTaskRunFormat  string  `json:"artifacts.taskrun.format,omitempty"`
	ArtifactsTaskRunStorage *string `json:"artifacts.taskrun.storage,omitempty"`
	ArtifactsTaskRunSigner  string  `json:"artifacts.taskrun.signer,omitempty"`

	// pipelinerun artifacts config
	ArtifactsPipelineRunFormat  string  `json:"artifacts.pipelinerun.format,omitempty"`
	ArtifactsPipelineRunStorage *string `json:"artifacts.pipelinerun.storage,omitempty"`
	ArtifactsPipelineRunSigner  string  `json:"artifacts.pipelinerun.signer,omitempty"`

	// oci artifacts config
	ArtifactsOCIFormat  string  `json:"artifacts.oci.format,omitempty"`
	ArtifactsOCIStorage *string `json:"artifacts.oci.storage,omitempty"`
	ArtifactsOCISigner  string  `json:"artifacts.oci.signer,omitempty"`

	// storage configs
	StorageGCSBucket             string `json:"storage.gcs.bucket,omitempty"`
	StorageOCIRepository         string `json:"storage.oci.repository,omitempty"`
	StorageOCIRepositoryInsecure *bool  `json:"storage.oci.repository.insecure,omitempty"`
	StorageDocDBURL              string `json:"storage.docdb.url,omitempty"`
	StorageGrafeasProjectID      string `json:"storage.grafeas.projectid,omitempty"`
	StorageGrafeasNoteID         string `json:"storage.grafeas.noteid,omitempty"`
	StorageGrafeasNoteHint       string `json:"storage.grafeas.notehint,omitempty"`

	// builder config
	BuilderID string `json:"builder.id,omitempty"`

	// x509 signer config
	X509SignerFulcioEnabled     *bool  `json:"signers.x509.fulcio.enabled,omitempty"`
	X509SignerFulcioAddr        string `json:"signers.x509.fulcio.address,omitempty"`
	X509SignerFulcioOIDCIssuer  string `json:"signers.x509.fulcio.issuer,omitempty"`
	X509SignerFulcioProvider    string `json:"signers.x509.fulcio.provider,omitempty"`
	X509SignerIdentityTokenFile string `json:"signers.x509.identity.token.file,omitempty"`
	X509SignerTUFMirrorURL      string `json:"signers.x509.tuf.mirror.url,omitempty"`

	// kms signer config
	KMSRef               string `json:"signers.kms.kmsref,omitempty"`
	KMSAuthAddress       string `json:"signers.kms.auth.address,omitempty"`
	KMSAuthToken         string `json:"signers.kms.auth.token,omitempty"`
	KMSAuthOIDCPath      string `json:"signers.kms.auth.oidc.path,omitempty"`
	KMSAuthOIDCRole      string `json:"signers.kms.auth.oidc.role,omitempty"`
	KMSAuthSpireSock     string `json:"signers.kms.auth.spire.sock,omitempty"`
	KMSAuthSpireAudience string `json:"signers.kms.auth.spire.audience,omitempty"`

	TransparencyConfigEnabled BoolValue `json:"transparency.enabled,omitempty"`
	TransparencyConfigURL     string    `json:"transparency.url,omitempty"`
}

type BoolValue string

func (bv *BoolValue) UnmarshalJSON(value []byte) error {
	var a string
	var b bool
	if err := json.Unmarshal(value, &a); err == nil {
		// no error, it's a string
		*bv = BoolValue(a)
		return nil
	} else if err := json.Unmarshal(value, &b); err == nil {
		// it is a boolean
		*bv = BoolValue(strconv.FormatBool(b))
		return nil
	}
	return fmt.Errorf("Invalid value")
}

func (bv BoolValue) MarshalJson() ([]byte, error) {
	return []byte(bv), nil
}

// TektonChainStatus defines the observed state of TektonChain
type TektonChainStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for TektonChain
	// +optional
	TektonInstallerSet string `json:"tektonInstallerSet,omitempty"`
}

// TektonChainList contains a list of TektonChain
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonChainList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonChain `json:"items"`
}
