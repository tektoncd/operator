package manifestival

import (
	"io"

	"github.com/manifestival/manifestival/sources"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Source interface {
	Parse() ([]unstructured.Unstructured, error)
}

// Implementations
var _ Source = Path("")
var _ Source = Recursive("")
var _ Source = Slice([]unstructured.Unstructured{})
var _ Source = reader{} // see Reader(io.Reader)

type Path string
type Recursive string
type Slice []unstructured.Unstructured

func Reader(r io.Reader) Source {
	return reader{r}
}

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
