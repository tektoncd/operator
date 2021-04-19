[![Build Status](https://github.com/manifestival/client-go-client/workflows/Build%20and%20Test/badge.svg)](https://github.com/manifestival/client-go-client/actions)

# client-go-client

A [client-go](https://github.com/kubernetes/client-go) implementation
of the [Manifestival](https://github.com/manifestival/manifestival)
`Client`.

Usage
-----

```go
import (
    mfc "github.com/manifestival/client-go-client"
    mf  "github.com/manifestival/manifestival"
    "k8s.io/client-go/rest"
)

func main() {
    var config *rest.Config = ...

    manifest, err := mfc.NewManifest("file.yaml", config)
    if err != nil {
        panic("Failed to load manifest")
    }
    manifest.Apply()

    // a slightly more complex example
    client, _ := mfc.NewClient(config)
    m, err := mf.ManifestFrom(mf.Recursive("dir/"), mf.UseClient(client))
    if err != nil {
        panic("Failed to load manifest")
    }
    m.Apply()
}
```

The `NewManifest` function in this library delegates to the function
of the same name in the `manifestival` package after constructing a
`manifestival.Client` implementation from the `*rest.Config`.
