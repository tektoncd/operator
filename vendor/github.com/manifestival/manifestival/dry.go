package manifestival

import (
	"encoding/json"

	jsonpatch "github.com/evanphx/json-patch/v5"
	"github.com/manifestival/manifestival/internal/patch"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/client-go/kubernetes/scheme"
)

type MergePatch map[string]interface{}

// DryRun returns a list of merge patches, either strategic or
// RFC-7386 for unregistered types, that show the effects of applying
// the manifest.
func (m Manifest) DryRun() ([]MergePatch, error) {
	diffs, err := m.diff()
	if err != nil {
		return nil, err
	}
	result := make([]MergePatch, len(diffs))
	for i, bytes := range diffs {
		if err := json.Unmarshal(bytes, &result[i]); err != nil {
			return nil, err
		}
	}
	return result, nil
}

// diff loads the resources in the manifest and computes their difference
func (m Manifest) diff() ([][]byte, error) {
	result := make([][]byte, 0, len(m.resources))
	for _, spec := range m.resources {
		current, err := m.Client.Get(&spec)
		if err != nil {
			if errors.IsNotFound(err) {
				// this resource will be created when applied
				jmp, _ := spec.MarshalJSON()
				result = append(result, jmp)
				continue
			}
			return nil, err
		}
		// ignore manifestival metadata by forcing it to match
		if anns := current.GetAnnotations(); anns != nil {
			if v, ok := anns["manifestival"]; ok {
				annotate(&spec, "manifestival", v)
			}
		}
		// create diff
		diff, err := patch.New(current, &spec)
		if err != nil {
			return nil, err
		}
		if diff == nil {
			// ignore things that won't change
			continue
		}
		// apply diff
		modified := current.DeepCopy()
		if err := diff.Merge(modified); err != nil {
			return nil, err
		}
		// Remove these fields so they'll be included in the patch
		current.SetAPIVersion("")
		current.SetKind("")
		current.SetName("")
		jmp, err := mergePatch(current, modified)
		if err != nil {
			return nil, err
		}
		result = append(result, jmp)
	}
	return result, nil
}

// mergePatch returns a 2-way merge patch
func mergePatch(orig, mod *unstructured.Unstructured) (_ []byte, err error) {
	var original, modified []byte
	if original, err = orig.MarshalJSON(); err != nil {
		return
	}
	if modified, err = mod.MarshalJSON(); err != nil {
		return
	}
	obj, err := scheme.Scheme.New(mod.GroupVersionKind())
	switch {
	case runtime.IsNotRegisteredError(err):
		return jsonpatch.CreateMergePatch(original, modified)
	case err != nil:
		return nil, err
	default:
		return strategicpatch.CreateTwoWayMergePatch(original, modified, obj)
	}
}
