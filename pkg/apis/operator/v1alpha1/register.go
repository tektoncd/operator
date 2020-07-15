// NOTE: Boilerplate only.  Ignore this file.

// Package v1alpha1 contains API Schema definitions for the operator v1alpha1 API group
// +k8s:deepcopy-gen=package,register
// +groupName=operator.tekton.dev
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// GroupName is the group of the API.
	GroupName = "operator.tekton.dev"

	// SchemaVersion is the current version of the API.
	SchemaVersion = "v1alpha1"
)

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func Resource(resource string) schema.GroupResource {
	return SchemeGroupVersion.WithResource(resource).GroupResource()
}

// addKnownTypes adds the set of types defined in this package to the supplied
// scheme.
func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypes(SchemeGroupVersion,
		&TektonPipeline{},
		&TektonPipelineList{},
		&TektonAddon{},
		&TektonAddonList{})
	metav1.AddToGroupVersion(s, SchemeGroupVersion)
	return nil
}

var (
	// SchemeGroupVersion is group version used to register these objects
	SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: SchemaVersion}
	// SchemeBuilder is used to add go types to the GroupVersionKind scheme
	SchemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme adds the API's types to the Scheme.
	AddToScheme = SchemeBuilder.AddToScheme
)
