package shipwrightbuild

// Well-known names used across the Shipwright Build install: the deployments and
// container the manifests ship, the operand label value that tags every applied
// resource, and the configmap the operator reads the component version from.
const (
	ControllerDeploymentName = "shipwright-build-controller"
	KindClusterBuildStrategy = "ClusterBuildStrategy"
	OperandName              = "shipwright-build"
	VersionConfigMap         = "shipwright-build"
	WebhookContainerName     = "shipwright-build-webhook"
	WebhookDeploymentName    = "shipwright-build-webhook"
	// WebhookSecretName / WebhookServiceName / WebhookSecretDuration describe the
	// TLS serving certificate the webhook needs; the operator provisions it since
	// the upstream manifests expect the secret to already exist.
	WebhookSecretName  = "shipwright-build-webhook-cert"
	WebhookServiceName = "shp-build-webhook"
	// WebhookVersionFlag is passed to the webhook so it advertises the version the
	// operator installed.
	WebhookVersionFlag = "--version"
)

var (
	ControllerLabels = map[string]string{
		"name": "shipwright-build",
	}
	WebhookLabels = map[string]string{
		"name": "shp-build-webhook",
	}
)
