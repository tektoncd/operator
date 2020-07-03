# Changelog

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/)


## [Unreleased]

### Changed

### Added

### Removed


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
[unreleased]: https://github.com/manifestival/manifestival/compare/v0.5.0...HEAD
[0.5.0]: https://github.com/manifestival/manifestival/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/manifestival/manifestival/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/manifestival/manifestival/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/manifestival/manifestival/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/manifestival/manifestival/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/manifestival/manifestival/compare/v0.0.0...v0.1.0
[0.0.0]: https://github.com/manifestival/manifestival/releases/tag/v0.0.0
