package openshift

const (
	KueueNameSpace                  = "openshift-kueue-operator"
	OperandOpenShiftPipelinesAddons = "openshift-pipelines-addons"
	OperandOpenShiftPipelineAsCode  = "openshift-pipeline-as-code"
	// NamespaceSCCAnnotation is used to set SCC for a given namespace
	NamespaceSCCAnnotation = "operator.tekton.dev/scc"
)
