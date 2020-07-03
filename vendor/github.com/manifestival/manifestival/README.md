# Manifestival

[![Build Status](https://travis-ci.org/manifestival/manifestival.svg?branch=master)](https://travis-ci.org/manifestival/manifestival)

Manifestival is a library for manipulating a set of unstructured
Kubernetes resources. Essentially, it enables you to toss a "bag of
YAML" at a k8s cluster.

It's sort of like embedding a simplified `kubectl` in your Go
application. You can load a manifest of resources from a variety of
sources, optionally transform/filter those resources, and then
apply/delete them to/from your k8s cluster.

See [CHANGELOG.md](CHANGELOG.md)

## Usage

### Client

Manifests require a `Client` implementation to interact with your k8s
API server. You have two choices:

- <https://github.com/manifestival/client-go-client>
- <https://github.com/manifestival/controller-runtime-client>

Once you have a client, create a manifest from some path to a YAML
doc. This could be a path to a file, directory, or URL. Other sources
are supported, too.

```go
manifest, err := NewManifest("/path/to/file.yaml", UseClient(client))
if err != nil {
    panic("Failed to load manifest")
}
```

It's the `Client` that enables you to persist the resources in your
manifest using `Apply` and remove them using `Delete`. You can even
invoke the manifest's `Client` directly.

```go
manifest.Apply()
manifest.Filter(NoCRDs).Delete()

u := manifest.Resources()[0]
u.SetName("foo")
manifest.Client.Create(&u)
```

Manifests are immutable once created, but you can create new instances
using the `Filter` and `Transform` functions.

### Filter

There are a few built-in `Predicates` for `Filter`, and you can easily
create your own. If you pass multiple, they're "AND'd" together, so
only resources matching every predicate will be included in the
returned manifest.

```go
m := manifest.Filter(ByLabel("foo", "bar"), ByGVK(gvk), NoCRDs)
```

Because the `Predicate` receives the resource by reference, any
changes you make to it will be reflected in the returned `Manifest`,
but _not_ in the one being filtered. Since errors are not in the
`Predicate` interface, you should limit changes to those that won't
error. For more complex mutations, use `Transform` instead.

### Transform

`Transform` will apply some function to every resource in your
manifest, so it's common for a `Transformer` function to include a
guard that simply returns if the unstructured resource isn't of
interest.

There are a few handy built-in `Transformers` provided as well.

```go
func updateDeployment(resource *unstructured.Unstructured) error {
    if resource.GetKind() != "Deployment" {
        return nil
    }
    // Either manipulate the Unstructured resource directly or...
    // convert it to a structured type...
    var deployment = &appsv1.Deployment{}
    if err := scheme.Scheme.Convert(resource, deployment, nil); err != nil {
        return err
    }

    // Now update the deployment!
    
    // If you converted it, convert it back, otherwise return nil
    return scheme.Scheme.Convert(deployment, resource, nil)
}

m, err := manifest.Transform(updateDeployment, InjectOwner(parent), InjectNamespace("foo"))
```

## Development

You know the drill.

    dep ensure -v
    go test -v ./...
