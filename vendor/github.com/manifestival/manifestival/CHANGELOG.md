# Changelog

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)


## [Unreleased]

### Changed

### Added

### Removed


## [0.7.0] - 2021-02-11

### Changed

- It is no longer possible to mutate manifest resources with the
  `Filter` method, since now only deep copies of each resource are
  passed to each `Predicate`. The only way to change a manifest's
  resources is via the `Transform` method. [#75](https://github.com/manifestival/manifestival/issues/75)
- Updated Golang version to 1.15.
- Updated Kubernetes dependencies to 1.19.7.
- Updated `github.com/evanphx/json-patch` to `v5.2.0`
- Updated `github.com/go-logr/logr` to `v0.4.0`


## [0.6.1] - 2020-08-19

### Added

- Support for generated names: if `metadata.generateName` is set and
  `metadata.name` is *not* set on any resource in a manifest, that resource will
  always be _created_ when the manifest is _applied_. [#65](https://github.com/manifestival/manifestival/issues/65)
- Support for CRD apiextensions.k8s.io/v1: CRD v1/v1beta has a difference 
  in the specification on the conversion webhook field, which needs to be 
  compatible in the InjectNamespace function.

### Changed

- Fixed the `In` predicate to not incorporate the API version in its comparison
  of manifest resources. Only Group, Kind, Namespace, and Name are used to test
  for equality. [#67](https://github.com/manifestival/manifestival/issues/67)
- After experimenting with dynamically constructing `Any` and `All`
  predicates, we decided to partially revert
  [#56](https://github.com/manifestival/manifestival/pull/56): `Any`
  and `All` no longer require at least one argument as it has become
  clear that `All()` should match `Everything` and `Any()` should
  match `Nothing`. [#69](https://github.com/manifestival/manifestival/issues/69)


## [0.6.0] - 2020-07-07

### Changed

- Migrated from [dep](https://github.com/golang/dep) to [go
  modules](https://blog.golang.org/using-go-modules)
  [#47](https://github.com/manifestival/manifestival/pull/47)
- Restored `FieldManager` option for creates and updates, essentially
  reverting [#17](https://github.com/manifestival/manifestival/issues/17).
  [#26](https://github.com/manifestival/manifestival/issues/26)
- Fixed the `InjectNamespace` transformer to properly update the
  `spec.conversion` field in a `CustomResourceDefinition`
  [#55](https://github.com/manifestival/manifestival/issues/55)
- Predicate changes: `None` was removed and replaced with `Not`, which
  only accepts a single predicate. `Any` and `All` now require at
  least one predicate since it wasn't clear how they should behave
  without one. [#56](https://github.com/manifestival/manifestival/pull/56)
- Fixed bug where manifestival wasn't deleting namespaces it created.
  (It should never delete a namespace it didn't create)
  [#61](https://github.com/manifestival/manifestival/issues/61)

### Added

- Introduced `Append` to the `Manifestival` interface. This enables
  the creation of new manifests from the concatenation of others. The
  resulting manifest retains the options, e.g. client and logger, of
  the receiver. [#41](https://github.com/manifestival/manifestival/issues/41)
- New fake `Client` to facilitate testing. Provides both a simple
  in-memory object store and easily-override-able stubs for all the
  `Client` functions: `Create`, `Update`, `Delete`, or `Get`
  [#43](https://github.com/manifestival/manifestival/pull/43)
- More [docs](README.md), including
  [godoc](https://godoc.org/github.com/manifestival/manifestival)
  [#42](https://github.com/manifestival/manifestival/pull/42)
- New filter `Predicate`, `In`, that returns true if a resource is in
  a given manifest, uniquely identified by GVK, namespace, and name
  [#50](https://github.com/manifestival/manifestival/pull/50)
- New filter `Predicate`, `ByAnnotation`, that does for annotations
  what `ByLabel` did for labels!
  [#52](https://github.com/manifestival/manifestival/pull/52)
- Defaulting the `FieldManager` for create/updates to "manifestival"
  to help reconcile changes in `metadata.managedFields`, in
  anticipation of server-side apply. [#64](https://github.com/manifestival/manifestival/pull/64)

### Removed

- Removed dependency on `k8s.io/kubernetes`. It was only used in a
  test, to verify a proper response to server-side validation errors,
  but 'go modules' doesn't distinguish test-only dependencies, and
  `k8s.io/kubernetes` was never intended to be consumed as a module,
  so we replicated the validation logic in the test itself.
  

## [0.5.0] - 2020-03-31

### Changed

- Renamed the `Replace` option to `Overwrite` to better match the
  behavior of the `kubectl apply` subcommand. Its default value is now
  true, which will cause `Apply` to "automatically resolve conflicts
  between the modified and live configuration by using values from the
  modified configuration". To override this behavior and have invalid
  patches return an error, call `Apply(Overwrite(false))` [#39](https://github.com/manifestival/manifestival/pull/39)
- Made the `None` filter variadic, accepting multiple `Predicates`,
  returning only those resources matching none of them. [#36](https://github.com/manifestival/manifestival/issues/36)
  

## [0.4.0] - 2020-03-11

### Added

- New `DryRun` function shows how applying the manifest will change
  the cluster. Its return value is a list of strategic merge patches
  [#29](https://github.com/manifestival/manifestival/pull/29)
- New `ByLabels` function which is similar to `ByLabel`, but it
  accepts multiple labels in a map and filters all resources that
  match any of them [#32](https://github.com/manifestival/manifestival/pull/32)

### Changed

- Reordered/renamed parameters in the `patch` package to be more
  consistent with its upstream functions.
- The `Patch` interface is now a struct
- Renamed `Patch.Apply` to `Patch.Merge`
- `Delete` may now return the errors that it was previously only
  logging. It will still ignore `NotFound` errors when the
  `IgnoreNotFound` option is true, of course.
- Fixed bug when transformers use `scheme.Scheme.Convert` to
  manipulate resources [#33](https://github.com/manifestival/manifestival/pull/33)


## [0.3.1] - 2020-02-26

### Changed

- Bugfix: set LastAppliedConfigAnnotation correctly on updates
  [#27](https://github.com/manifestival/manifestival/issues/27)


## [0.3.0] - 2020-02-25

### Added

- Introduced `All` and `Any` predicates, implementing `Filter` in
  terms of the former
- A new `ApplyOption` called `Replace` that defaults to false. Can be
  used to force a replace update instead of a merge patch when
  applying a manifest, e.g. `m.Apply(Replace(true))` or
  `m.Apply(ForceReplace)` [#23](https://github.com/manifestival/manifestival/issues/23)

### Removed

- `ConfigMaps` are no longer handled specially when applying manifests

### Changed

- Renamed `Complement` to `None`, `JustCRDs` to `CRDs`, and `NotCRDs`
  to `NoCRDs`.


## [0.2.0] - 2020-02-21

### Added

- Introduced the `Source` interface, enabling the creation of a
  Manifest from any source [#11](https://github.com/manifestival/manifestival/pull/11)
- Added a `ManifestFrom` constructor to complement `NewManifest`,
  which now only works for paths to files, directories, and URL's
- Use `ManifestFrom(Recursive("dirname/"))` to create a manifest from
  a recursive directory search for yaml files.
- Use `ManifestFrom(Slice(v))` to create a manifest from any `v` of type
  `[]unstructured.Unstructured`
- Use `ManifestFrom(Reader(r))` to create a manifest from any `r` of
  type `io.Reader`
- Introduced a new `Filter` function in the `Manifestival` interface
  that returns a subset of resources matching one or more `Predicates`
- Convenient predicates provided: `ByName`, `ByKind`, `ByLabel`,
  `ByGVK`, `Complement`, `JustCRDs`, and `NotCRDs`

### Removed

- In order to support k8s versions <1.14, the code no longer
  references the `FieldManager` field of either `metav1.CreateOptions`
  or `metav1.UpdateOptions`. This only affects `client-go` clients who
  set `FieldManager` on their creates/updates; `controller-runtime`
  clients aren't affected.
  [#17](https://github.com/manifestival/manifestival/issues/17)

### Changed

- Removed the "convenience" functions from the `Manifestival`
  interface and renamed `ApplyAll` and `DeleteAll` to `Apply` and
  `Delete`, respectively. [#14](https://github.com/manifestival/manifestival/issues/14)
- The `Manifest` struct's `Client` is now public, so the handy `Get`
  and `Delete` functions formerly in the `Manifestival` interface can
  now be invoked directly on any manifest's `Client` member.
- Manifests created from a recursive directory search are now only
  possible via the new `ManifestFrom` constructor. The `NewManifest`
  constructor no longer supports a `recursive` option.
- Moved the path/yaml parsing logic into its own `sources` package to
  reduce the exported names in the `manifestival` package.
- Split the `ClientOption` type into `ApplyOption` and `DeleteOption`,
  adding `IgnoreNotFound` to the latter, thereby enabling `Client`
  implementations to honor it, simplifying delete logic for users
  invoking the `Client` directly. All `ApplyOptions` apply to both
  creates and updates
  [#12](https://github.com/manifestival/manifestival/pull/12)
- The `Manifest` struct's `Resources` member is no longer public.
  Instead, a `Manifest.Resources()` function is provided to return a
  deep copy of the manifest's resources, if needed.
- `Transform` now returns a `Manifest` by value, like `Filter`. This
  is a stronger indicator of immutability and conveniently matches the
  return type of `NewManifest`. [#18](https://github.com/manifestival/manifestival/issues/18)
- The receiver types for the `Apply`, `Delete`, and `Resources`
  methods on `Manifest` are now values, enabling convenient chaining
  of calls involving `Filter`, for example. [#21](https://github.com/manifestival/manifestival/issues/21)


## [0.1.0] - 2020-02-17

### Changed

- Factored client API calls into a `Client` interface; implementations
  for [controller-runtime] and [client-go] reside in separate repos
  within this org [#4](https://github.com/manifestival/manifestival/issues/4)
- Introduced `Option` and `ClientOption` types, enabling golang's
  "functional options" pattern for both Manifest creation and `Client`
  interface options, respectively [#6](https://github.com/manifestival/manifestival/issues/6)
- Transforms are now immutable, a feature developed in the old
  `client-go` branch
- Except for `ConfigMaps`, manifest resources are now applied using
  kubectl's strategic 3-way merging, a feature also developed in the
  old `client-go` branch. `ConfigMaps` use a JSON merge patch,
  which is essentially a "replace"
- Other small fixes from the old `client-go` branch have also been
  merged 


## [0.0.0] - 2020-01-11

### Changed

- This release represents the move from this project's original home,
  https://github.com/jcrossley3/manifestival.
- There were no releases in the old repo, just two branches: `master`,
  based on the `controller-runtime` client API, and `client-go`, based
  on the `client-go` API. 


[controller-runtime]: https://github.com/manifestival/controller-runtime-client
[client-go]: https://github.com/manifestival/client-go-client
[Unreleased]: https://github.com/manifestival/manifestival/compare/v0.7.0...HEAD
[0.7.0]: https://github.com/manifestival/manifestival/compare/v0.6.1...v0.7.0
[0.6.1]: https://github.com/manifestival/manifestival/compare/v0.6.0...v0.6.1
[0.6.0]: https://github.com/manifestival/manifestival/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/manifestival/manifestival/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/manifestival/manifestival/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/manifestival/manifestival/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/manifestival/manifestival/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/manifestival/manifestival/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/manifestival/manifestival/compare/v0.0.0...v0.1.0
[0.0.0]: https://github.com/manifestival/manifestival/releases/tag/v0.0.0
