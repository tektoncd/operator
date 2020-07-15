package setup

const (
	DefaultTargetNs = "tekton-pipelines"

	// TektonPipelineCRName is the name of the TektonPipeline CR
	TektonPipelineCRName = "cluster"

	// Name of the pipeline controller deployment
	PipelineControllerName = "tekton-pipelines-controller"

	// Name of the pipeline webhook deployment
	PipelineWebhookName = "tekton-pipelines-webhook"
)
