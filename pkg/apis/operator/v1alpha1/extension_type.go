package v1alpha1

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ExtensionWapper define the attributes used by Extension.
// Extension is stuff like plugin, to make add/remove feature easier.
// +k8s:openapi-gen=true
type ExtensionWapper struct {
	// Registry is used for image replacement
	Registry *Registry `json:"registry,omitempty"`
}

// Registry defines image overrides of tekton images.
// The override values are specific to each tekton deployment.
// +k8s:openapi-gen=true
type Registry struct {
	// A map of a container name or arg key to the full image location of the individual tekton container.
	// +optional
	Override map[string]string `json:"override,omitempty"`
}
