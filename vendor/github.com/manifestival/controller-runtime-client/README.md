# controller-runtime-client

A [controller-runtime](https://github.com/kubernetes-sigs/controller-runtime)
implementation of the
[Manifestival](https://github.com/manifestival/manifestival) `Client`.

Usage
-----

```go
import (
    mfc "github.com/manifestival/controller-runtime-client"
    mf  "github.com/manifestival/manifestival"
    "sigs.k8s.io/controller-runtime/pkg/client"
)

func main() {
    var client client.Client = ...
    
    manifest, err := mfc.NewManifest("file.yaml", client)
    if err != nil {
        panic("Failed to load manifest")
    }
    manifest.Apply()
    
    // a slightly more complex example
    m, err := mf.ManifestFrom(mf.Recursive("dir/"), mf.UseClient(mfc.NewClient(client)))
    if err != nil {
        panic("Failed to load manifest")
    }
    m.Apply()
}
```

The `NewManifest` function in this library delegates to the function
of the same name in the `manifestival` package after constructing a
`manifestival.Client` implementation from the `client.Client`.
