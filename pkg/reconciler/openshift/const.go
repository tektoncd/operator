package openshift

const (
	OperandOpenShiftPipelinesAddons = "openshift-pipelines-addons"
	OperandOpenShiftPipelineAsCode  = "openshift-pipeline-as-code"
	// NamespaceSCCAnnotation is used to set SCC for a given namespace
	NamespaceSCCAnnotation = "operator.tekton.dev/scc"

	// RbacProvisioningMaxConcurrentCalls is used to set a go routine pool size when
	// we reconcile namespaces and do rbac provisonning
	RbacProvisioningMaxConcurrentCalls = "OCP_RBAC_MAX_CONCURRENT_CALLS"
)
