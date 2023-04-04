/*
Copyright 2021 The Tekton Authors

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
	_ TektonComponent     = (*TektonResult)(nil)
	_ TektonComponentSpec = (*TektonResultSpec)(nil)
)

// TektonResult is the Schema for the tektonresults API
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
type TektonResult struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TektonResultSpec   `json:"spec,omitempty"`
	Status TektonResultStatus `json:"status,omitempty"`
}

// GetSpec implements TektonComponent
func (tp *TektonResult) GetSpec() TektonComponentSpec {
	return &tp.Spec
}

// GetStatus implements TektonComponent
func (tp *TektonResult) GetStatus() TektonComponentStatus {
	return &tp.Status
}

// TektonResultSpec defines the desired state of TektonResult
type TektonResultSpec struct {
	CommonSpec           `json:",inline"`
	ResultsAPIProperties `json:",inline"`
}

// ResultsAPIProperties defines the fields which are configurable for
// Results API server config
type ResultsAPIProperties struct {
	DBUser                string `JSON:"db_user,omitempty"`
	DBPassword            string `json:"db_password,omitempty"`
	DBHost                string `json:"db_host,omitempty"`
	DBPort                int64  `json:"db_port,omitempty"`
	DBSSLMode             string `json:"db_sslmode,omitempty"`
	DBEnableAutoMigration bool   `json:"db_enable_auto_migration,omitempty"`
	LogLevel              string `json:"log_level,omitempty"`
	LogsAPI               bool   `json:"logs_api,omitempty"`
	LogsType              string `json:"logs_type,omitempty"`
	LogsBufferSize        int64  `json:"logs_buffer_size,omitempty"`
	LogsPath              string `json:"logs_path,omitempty"`
	TLSHostnameOverride   string `json:"tls_hostname_override,omitempty"`
	NoAuth                bool   `json:"no_auth,omitempty"`
	S3BucketName          string `json:"s3_bucket_name,omitempty"`
	S3Endpoint            string `json:"s3_endpoint,omitempty"`
	S3HostnameImmutable   bool   `json:"s3_hostname_immutable,omitempty"`
	S3Region              string `json:"s3_region,omitempty"`
	S3AccessKeyID         string `json:"s3_access_key_id,omitempty"`
	S3SecretAccessKey     string `json:"s3_secret_access_key,omitempty"`
	S3MultiPartSize       int64  `json:"s3_multi_part_size,omitempty"`
	LoggingPVCName        string `json:"logging_pvc_name"`
}

// TektonResultStatus defines the observed state of TektonResult
type TektonResultStatus struct {
	duckv1.Status `json:",inline"`

	// The version of the installed release
	// +optional
	Version string `json:"version,omitempty"`

	// The current installer set name for TektonResult
	// +optional
	TektonInstallerSet string `json:"tektonInstallerSet,omitempty"`
}

func (in *TektonResultStatus) MarkPreReconcilerFailed(s string) {
	//TODO implement me
	panic("implement me")
}

func (in *TektonResultStatus) MarkPostReconcilerFailed(s string) {
	//TODO implement me
	panic("implement me")
}

// TektonResultsList contains a list of TektonResult
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonResultList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonResult `json:"items"`
}
