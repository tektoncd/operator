package sources

import (
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

// decode consumes the given reader and parses its contents as YAML.
func Decode(reader io.Reader) ([]unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLToJSONDecoder(reader)
	objs := []unstructured.Unstructured{}
	var err error
	for {
		out := unstructured.Unstructured{}
		err = decoder.Decode(&out)
		if err == io.EOF {
			break
		}
		if err != nil || len(out.Object) == 0 {
			continue
		}
		objs = append(objs, out)
	}
	if err != io.EOF {
		return nil, err
	}
	return objs, nil
}
