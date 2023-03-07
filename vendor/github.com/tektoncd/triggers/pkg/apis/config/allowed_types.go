package config

import (
	pipelinev1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	customrunsv1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	pipelinev1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	pipelineresourcev1alpha1 "github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var Decoder runtime.Decoder

// TODO(dibyom): We should have a way of configuring this instead of an init function?
func init() {
	scheme := runtime.NewScheme()
	utilruntime.Must(customrunsv1alpha1.AddToScheme(scheme))
	utilruntime.Must(pipelineresourcev1alpha1.AddToScheme(scheme))
	utilruntime.Must(pipelinev1beta1.AddToScheme(scheme))
	utilruntime.Must(pipelinev1.AddToScheme(scheme))
	codec := serializer.NewCodecFactory(scheme)
	Decoder = codec.UniversalDecoder(
		pipelineresourcev1alpha1.SchemeGroupVersion, // customrunsv1alpha1 share the same SchemeGroupVersion
		pipelinev1beta1.SchemeGroupVersion,
		pipelinev1.SchemeGroupVersion,
	)
}

// EnsureAllowedType returns nil if the resourceTemplate has an apiVersion
// and kind field set to one of the allowed ones.
func EnsureAllowedType(rt runtime.RawExtension) error {
	_, err := runtime.Decode(Decoder, rt.Raw)
	return err
}

var (
	AllowedPipelineTypes = map[string][]string{
		"v1alpha1": {"pipelineresources", "pipelineruns", "taskruns", "pipelines", "clustertasks", "tasks", "conditions", "runs"},
		"v1beta1":  {"pipelineruns", "taskruns", "pipelines", "clustertasks", "tasks", "customruns"},
		"v1":       {"pipelineruns", "taskruns", "pipelines", "tasks"},
	}
	AllowedTriggersTypes = map[string][]string{
		"v1alpha1": {"clusterinterceptors", "interceptors"},
		"v1beta1":  {"clustertriggerbindings", "eventlisteners", "triggerbindings", "triggers", "triggertemplates"},
	}
)
