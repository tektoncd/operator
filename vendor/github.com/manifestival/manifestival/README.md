# Manifestival

[![Build Status](https://travis-ci.org/manifestival/manifestival.svg?branch=master)](https://travis-ci.org/manifestival/manifestival)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/manifestival/manifestival)](https://pkg.go.dev/github.com/manifestival/manifestival)

Manifestival is a library for manipulating a set of unstructured
Kubernetes resources. Essentially, it enables you to toss a "bag of
YAML" at a k8s cluster.

It's sort of like embedding a simplified `kubectl` in your Go
application. You can load a manifest of resources from a variety of
sources, optionally transform/filter those resources, and then
apply/delete them to/from your k8s cluster.

See [CHANGELOG.md](CHANGELOG.md)

* [Creating Manifests](#creating-manifests)
  * [Sources](#sources)
  * [Append](#append)
  * [Filter](#filter)
  * [Transform](#transform)
* [Applying Manifests](#applying-manifests)
  * [Client](#client)
    * [fake.Client](#fakeclient)
  * [Logging](#logging)
  * [Apply](#apply)
  * [Delete](#delete)
  * [DryRun](#dryrun)


## Creating Manifests

Manifests may be constructed from a variety of sources. Once created,
they are immutable, but new instances may be created from them using
their [Append], [Filter] and [Transform] functions.

The typical way to create a manifest is by calling `NewManifest`

```go
manifest, err := NewManifest("/path/to/file.yaml")
```

But `NewManifest` is just a convenience function that calls
`ManifestFrom` with a `Path`, an implementation of `Source`.

### Sources

A manifest is created by passing an implementation of the [Source]
interface to the `ManifestFrom` function. Here are the built-in types
that implement `Source`:

* `Path`
* `Recursive`
* `Slice`
* `Reader`

The `Path` source is the most versatile. It's a string representing
the location of some YAML content in many possible forms: a file, a
directory of files, a URL, or a comma-delimited list of any of those
things, all of which will be combined into a single manifest.

```go
// Single file
m, err := ManifestFrom(Path("/path/to/file.yaml"))

// All files in a directory
m, err := ManifestFrom(Path("/path/to/dir"))

// A remote URL
m, err := ManifestFrom(Path("http://site.com/manifest.yaml"))

// All of the above
m, err := ManifestFrom(Path("/path/to/file.yaml,/path/to/dir,http://site.com/manifest.yaml"))
```

`Recursive` works exactly like `Path` except that directories are
searched recursively.

The `Slice` source enables the creation of a manifest from an existing
slice of `[]unstructured.Unstructured`. This is helpful for testing
and, combined with the [Resources] accessor, facilitates more
sophisticated combinations of manifests, e.g. a crude form of
[Append](#append):

```go
m3, _ := ManifestFrom(Slice(append(m1.Resources(), m2.Resources()...)))
```

And `Reader` is a function that takes an `io.Reader` and returns a
`Source` from which valid YAML is expected.

### Append

The `Append` function enables the creation of new manifests from the
concatenation of others. The resulting manifest retains the options,
e.g. client and logger, of the receiver. For example,

```go
core, _ := NewManifest(path, UseLogger(logger), UseClient(client))
istio, _ := NewManifest(pathToIstio)
kafka, _ := NewManifest(pathToKafka)

manifest := core.Append(istio, kafka)
```

### Filter

[Filter] returns a new Manifest containing only the resources for
which _all_ passed predicates return true. A [Predicate] is a function
that takes an `Unstructured` resource and returns a bool indicating
whether the resource should be included in the filtered results.

There are a few built-in predicates and some helper functions for
creating your own:

* `All` returns a `Predicate` that returns true unless any of its
  arguments returns false
* `Everything` is equivalent to `All()`
* `Any` returns a `Predicate` that returns false unless any of its
  arguments returns true
* `Nothing` is equivalent to `Any()`
* `Not` negates its argument, returning false if its argument returns
  true
* `ByName`, `ByKind`, `ByLabel`, `ByAnnotation`, and `ByGVK` filter
  resources by their respective attributes.
* `CRDs` and its complement `NoCRDs` are handy filters for
  `CustomResourceDefinitions`
* `In` can be used to find the "intersection" of two manifests

```go
clusterRBAC := Any(ByKind("ClusterRole"), ByKind("ClusterRoleBinding"))
namespaceRBAC := Any(ByKind("Role"), ByKind("RoleBinding"))
rbac := Any(clusterRBAC, namespaceRBAC)

theRBAC := manifest.Filter(rbac)
theRest := manifest.Filter(Not(rbac))

// Find all resources named "controller" w/label 'foo=bar' that aren't CRD's
m := manifest.Filter(ByLabel("foo", "bar"), ByName("controller"), NoCRDs)
```

Because the `Predicate` receives the resource by reference, any
changes you make to it will be reflected in the returned `Manifest`,
but _not_ in the one being filtered -- manifests are immutable. Since
errors are not in the `Predicate` interface, you should limit changes
to those that won't error. For more complex mutations, use `Transform`
instead.


### Transform

[Transform] will apply some function to every resource in your
manifest, and return a new Manifest with the results. It's common for
a [Transformer] function to include a guard that simply returns if the
unstructured resource isn't of interest.

There are a few useful transformers provided, including
`InjectNamespace` and `InjectOwner`. An example should help to
clarify:

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


## Applying Manifests

Persisting manifests is accomplished via the [Apply] and [Delete]
functions of the [Manifestival] interface, and though [DryRun] doesn't
change anything, it does query the API Server. Therefore all of these
functions require a [Client].

### Client

Manifests require a [Client] implementation to interact with a k8s API
server. There are currently two alternatives:

- <https://github.com/manifestival/client-go-client>
- <https://github.com/manifestival/controller-runtime-client>

To apply your manifest, you'll need to provide a client when you
create it with the `UseClient` functional option, like so:

```go
manifest, err := NewManifest("/path/to/file.yaml", UseClient(client))
if err != nil {
    panic("Failed to load manifest")
}
```

It's the `Client` that enables you to persist the resources in your
manifest using `Apply`, remove them using `Delete`, compute
differences using `DryRun`, and occasionally it's even helpful to
invoke the manifest's `Client` directly...

```go
manifest.Apply()
manifest.Filter(NoCRDs).Delete()

u := manifest.Resources()[0]
u.SetName("foo")
manifest.Client.Create(&u)
```

#### fake.Client

The [fake] package includes a fake `Client` with stubs you can easily
override in your unit tests. For example,

```go
func verifySomething(t *testing.T, expected *unstructured.Unstructured) {
    client := fake.Client{
        fake.Stubs{
            Create: func(u *unstructured.Unstructured) error {
                if !reflect.DeepEqual(u, expected) {
                    t.Error("You did it wrong!")
                }
                return nil
            },
        },
    }
    manifest, _ := NewManifest("testdata/some.yaml", UseClient(client))
    callSomethingThatUltimatelyAppliesThis(manifest)
}
```

There is also a convenient `New` function that returns a
fully-functioning fake Client by overriding the stubs to persist the
resources in a map. Here's an example using it to test the `DryRun`
function: 

```go
client := fake.New()
current, _ := NewManifest("testdata/current.yaml", UseClient(client))
current.Apply()
modified, _ := NewManifest("testdata/modified.yaml", UseClient(client))
diffs, err := modified.DryRun()
```

### Logging

By default, Manifestival logs nothing, but it will happily log its
actions if you pass it a [logr.Logger] via its `UseLogger` functional
option, like so:

```go
m, _ := NewManifest(path, UseLogger(log.WithName("manifestival")), UseClient(c))
```

### Apply

[Apply] will persist every resource in the manifest to the cluster. It
will invoke either `Create` or `Update` depending on whether the
resource already exists. And if it does exist, the same 3-way
[strategic merge patch] used by `kubectl apply` will be applied. And
the same annotation used by `kubectl` to record the resource's
previous configuration will be updated, too.

The following functional options are supported, all of which map to
either the k8s `metav1.CreateOptions` and `metav1.UpdateOptions`
fields or `kubectl apply` flags:

* `Overwrite` [true] resolve any conflicts in favor of the manifest
* `FieldManager` the name of the actor applying changes
* `DryRunAll` if present, changes won't persist

### Delete

[Delete] attempts to delete all the manifest's resources in reverse
order. Depending on the resources' owner references, race conditions
with the k8s garbage collector may occur, and by default `NotFound`
errors are silently ignored.

The following functional options are supported, all except
`IgnoreNotFound` mirror the k8s `metav1.DeleteOptions`:

* `IgnoreNotFound` [true] silently ignore any `NotFound` errors 
* `GracePeriodSeconds` the number of seconds before the object should be deleted
* `Preconditions` must be fulfilled before a deletion is carried out
* `PropagationPolicy` whether and how garbage collection will be performed

### DryRun

[DryRun] returns a list of JSON merge patches that show the effects of
applying the manifest without modifying the live system. Each item in
the returned list is valid content for the `kubectl patch` command.


[Resources]: https://godoc.org/github.com/manifestival/manifestival#Manifest.Resources
[Source]: https://godoc.org/github.com/manifestival/manifestival#Source
[Manifestival]: https://godoc.org/github.com/manifestival/manifestival#Manifestival
[Append]: https://godoc.org/github.com/manifestival/manifestival#Manifest.Append
[Filter]: https://godoc.org/github.com/manifestival/manifestival#Manifest.Filter
[Transform]: https://godoc.org/github.com/manifestival/manifestival#Manifest.Transform
[Apply]: https://godoc.org/github.com/manifestival/manifestival#Manifest.Apply
[Delete]: https://godoc.org/github.com/manifestival/manifestival#Manifest.Delete
[DryRun]: https://godoc.org/github.com/manifestival/manifestival#Manifest.DryRun
[Predicate]: https://godoc.org/github.com/manifestival/manifestival#Predicate
[Client]: https://godoc.org/github.com/manifestival/manifestival#Client
[Transformer]: https://godoc.org/github.com/manifestival/manifestival#Transformer
[logr.Logger]: https://github.com/go-logr/logr
[fake]: https://godoc.org/github.com/manifestival/manifestival/fake
[strategic merge patch]: https://kubernetes.io/docs/tasks/manage-kubernetes-objects/declarative-config/#merge-patch-calculation
