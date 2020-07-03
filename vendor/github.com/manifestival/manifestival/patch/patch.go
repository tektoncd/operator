package patch

import (
	"bytes"

	jsonpatch "github.com/evanphx/json-patch"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
)

type Patch struct {
	patch  []byte
	schema strategicpatch.LookupPatchMeta
}

// Attempts to create a 3-way strategic JSON merge patch. Falls back
// to RFC-7386 if object's type isn't registered
func New(curr, mod *unstructured.Unstructured) (_ *Patch, err error) {
	var original, modified, current []byte
	original = getLastAppliedConfig(curr)
	if modified, err = mod.MarshalJSON(); err != nil {
		return
	}
	if current, err = curr.MarshalJSON(); err != nil {
		return
	}
	obj, err := scheme.Scheme.New(mod.GroupVersionKind())
	switch {
	case runtime.IsNotRegisteredError(err):
		return createJsonMergePatch(original, modified, current)
	case err != nil:
		return
	default:
		return createStrategicMergePatch(original, modified, current, obj)
	}
}

// Apply the patch to the resource
func (p *Patch) Merge(obj *unstructured.Unstructured) (err error) {
	var current, result []byte
	if current, err = obj.MarshalJSON(); err != nil {
		return
	}
	if p.schema == nil {
		result, err = jsonpatch.MergePatch(current, p.patch)
	} else {
		result, err = strategicpatch.StrategicMergePatchUsingLookupPatchMeta(current, p.patch, p.schema)
	}
	if err != nil {
		return
	}
	return obj.UnmarshalJSON(result)
}

func (p *Patch) String() string {
	return string(p.patch)
}

func createJsonMergePatch(original, modified, current []byte) (*Patch, error) {
	patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(original, modified, current)
	return create(patch, nil), err
}

func createStrategicMergePatch(original, modified, current []byte, obj runtime.Object) (*Patch, error) {
	schema, err := strategicpatch.NewPatchMetaFromStruct(obj)
	if err != nil {
		return nil, err
	}
	patch, err := strategicpatch.CreateThreeWayMergePatch(original, modified, current, schema, true)
	return create(patch, schema), err
}

func create(patch []byte, schema strategicpatch.LookupPatchMeta) *Patch {
	if bytes.Equal(patch, []byte("{}")) {
		return nil
	}
	return &Patch{patch, schema}
}

func getLastAppliedConfig(obj *unstructured.Unstructured) []byte {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		return nil
	}
	return []byte(annotations[v1.LastAppliedConfigAnnotation])
}
