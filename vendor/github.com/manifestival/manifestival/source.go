package manifestival

import (
	"io"

	"github.com/manifestival/manifestival/internal/sources"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Source is the interface through which all Manifests are created.
type Source interface {
	Parse() ([]unstructured.Unstructured, error)
}

// Path is a Source represented as a comma-delimited list of files,
// directories, and URL's
type Path string

// Recursive is identical to Path, but dirs are searched recursively
type Recursive string

// Slice is a Source comprised of existing objects
type Slice []unstructured.Unstructured

// Reader takes an io.Reader from which YAML content is expected
func Reader(r io.Reader) Source {
	return reader{r}
}

var _ Source = Path("")
var _ Source = Recursive("")
var _ Source = Slice([]unstructured.Unstructured{})
var _ Source = reader{} // see Reader(io.Reader)

func (p Path) Parse() ([]unstructured.Unstructured, error) {
	return sources.Parse(string(p), false)
}

func (r Recursive) Parse() ([]unstructured.Unstructured, error) {
	return sources.Parse(string(r), true)
}

func (s Slice) Parse() ([]unstructured.Unstructured, error) {
	return []unstructured.Unstructured(s), nil
}

func (r reader) Parse() ([]unstructured.Unstructured, error) {
	return sources.Decode(r.real)
}

type reader struct {
	real io.Reader
}
