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
	LokiStackProperties  `json:",inline"`
}

type LokiStackProperties struct {
	LokiStackName      string `json:"loki_stack_name,omitempty"`
	LokiStackNamespace string `json:"loki_stack_namespace,omitempty"`
}

// ResultsAPIProperties defines the fields which are configurable for
// Results API server config
type ResultsAPIProperties struct {
	DBHost                string `json:"db_host,omitempty"`
	DBPort                *int64 `json:"db_port,omitempty"`
	DBName                string `json:"db_name,omitempty"`
	DBSSLMode             string `json:"db_sslmode,omitempty"`
	DBSSLRootCert         string `json:"db_sslrootcert,omitempty"`
	DBEnableAutoMigration *bool  `json:"db_enable_auto_migration,omitempty"`
	ServerPort            *int64 `json:"server_port,omitempty"`
	PrometheusPort        *int64 `json:"prometheus_port,omitempty"`
	PrometheusHistogram   *bool  `json:"prometheus_histogram,omitempty"`
	LogLevel              string `json:"log_level,omitempty"`
	LogsAPI               *bool  `json:"logs_api,omitempty"`
	LogsType              string `json:"logs_type,omitempty"`
	LogsBufferSize        *int64 `json:"logs_buffer_size,omitempty"`
	LogsPath              string `json:"logs_path,omitempty"`
	TLSHostnameOverride   string `json:"tls_hostname_override,omitempty"`
	AuthDisable           *bool  `json:"auth_disable,omitempty"`
	AuthImpersonate       *bool  `json:"auth_impersonate,omitempty"`
	LoggingPVCName        string `json:"logging_pvc_name,omitempty"`
	GcsBucketName         string `json:"gcs_bucket_name,omitempty"`
	StorageEmulatorHost   string `json:"storage_emulator_host,omitempty"`
	// name of the secret used to get S3 credentials and
	// pass it as environment variables to the "tekton-results-api" deployment under "api" container
	SecretName         string `json:"secret_name,omitempty"`
	GCSCredsSecretName string `json:"gcs_creds_secret_name,omitempty"`
	GCSCredsSecretKey  string `json:"gcs_creds_secret_key,omitempty"`
	IsExternalDB       bool   `json:"is_external_db"`

	LoggingPluginTLSVerificationDisable bool   `json:"logging_plugin_tls_verification_disable,omitempty"`
	LoggingPluginProxyPath              string `json:"logging_plugin_proxy_path,omitempty"`
	LoggingPluginAPIURL                 string `json:"logging_plugin_api_url,omitempty"`
	LoggingPluginTokenPath              string `json:"logging_plugin_token_path,omitempty"`
	LoggingPluginNamespaceKey           string `json:"logging_plugin_namespace_key,omitempty"`
	LoggingPluginStaticLabels           string `json:"logging_plugin_static_labels,omitempty"`
	LoggingPluginCACert                 string `json:"logging_plugin_ca_cert,omitempty"`
	LoggingPluginForwarderDelayDuration *uint  `json:"logging_plugin_forwarder_delay_duration,omitempty"`
	LoggingPluginQueryLimit             *uint  `json:"logging_plugin_query_limit,omitempty"`
	LoggingPluginQueryParams            string `json:"logging_plugin_query_params,omitempty"`
	// Options holds additions fields and these fields will be updated on the manifests
	Options AdditionalOptions `json:"options"`
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

func (trs *TektonResultStatus) MarkPreReconcilerFailed(msg string) {
	trs.MarkNotReady("PreReconciliation failed")
	resultsCondSet.Manage(trs).MarkFalse(
		PreReconciler,
		"Error",
		msg)
}

func (trs *TektonResultStatus) MarkPostReconcilerFailed(msg string) {
	trs.MarkNotReady("PostReconciliation failed")
	resultsCondSet.Manage(trs).MarkFalse(
		PostReconciler,
		"Error",
		msg)
}

// TektonResultsList contains a list of TektonResult
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TektonResultList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TektonResult `json:"items"`
}
