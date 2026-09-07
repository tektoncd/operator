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

package sharedresource

const (
	// NodeDaemonSetName is the name of the DaemonSet that runs the
	// Shared Resource Driver node.
	NodeDaemonSetName = "shared-resource-csi-driver-node"
	// WebhookContainerName is the name of the container that runs the
	// Shared Resource webhook.
	WebhookContainerName = "shared-resource-csi-driver-webhook"
	// WebhookDeploymentName is the name of the Deployment that runs the
	// Shared Resource webhook.
	WebhookDeploymentName = "shared-resource-csi-driver-webhook"
	// OperandName is the operand name label attached to the installed manifests.
	OperandName = "shared-resource"
	// VersionConfigMap is the ConfigMap that carries the installed component
	// version. It must match v1alpha1.SharedResourceResourceName so that the
	// init controller selects the shared-resource payload.
	VersionConfigMap = "shared-resource"
	// ServiceName is the service used to for CSI driver node.
	ServiceName = "shared-resource-csi-driver-node"
	// ServiceMonitorName is the name of the Service Monitor
	ServiceMonitorName = "shared-resource-csi-driver-node"
)

const (
	// tlsMinVersion and tlsCipherSuites are the webhook command-line flags used
	// to pin the minimum TLS version and allowed cipher suites to the cluster-wide
	// TLS policy resolved from TektonConfig.
	tlsMinVersion   = "--tls-min-version"
	tlsCipherSuites = "--tls-cipher-suites"
)

var (
	NodeLabels = map[string]string{
		"app": "shared-resource-csi-driver-node",
	}
	WebhookLabels = map[string]string{
		"name": "shared-resource-csi-driver-webhook",
	}
)
