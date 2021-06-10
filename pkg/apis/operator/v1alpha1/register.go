/*
Copyright 2020 The Tekton Authors

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
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// GroupName is the group of the API.
	GroupName = "operator.tekton.dev"

	// SchemaVersion is the current version of the API.
	SchemaVersion = "v1alpha1"

	// KindTektonPipeline is the Kind of Tekton Pipeline in a GVK context.
	KindTektonPipeline = "TektonPipeline"

	// KindTektonTrigger is the Kind of Tekton Trigger in a GVK context.
	KindTektonTrigger = "TektonTrigger"

	// KindTektonDashboard is the Kind of Tekton Dashboard in a GVK context.
	KindTektonDashboard = "TektonDashboard"

	// KindTektonAddon is the Kind of Tekton Addon in a GVK context.
	KindTektonAddon = "TektonAddon"

	// KindTektonConfig is the Kind of Tekton Config in a GVK context.
	KindTektonConfig = "TektonConfig"

	// KindTektonResult is the Kind of Tekton Config in a GVK context.
	KindTektonResult = "TektonResult"
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
		&TektonTrigger{},
		&TektonTriggerList{},
		&TektonDashboard{},
		&TektonDashboardList{},
		&TektonAddon{},
		&TektonAddonList{},
		&TektonConfig{},
		&TektonConfigList{},
		&TektonResult{},
		&TektonResultList{},
	)
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
