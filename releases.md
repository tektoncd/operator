# Tekton Operator Releases

## Release Frequency

Tekton Operator follows the Tekton community [release policy][release-policy]
as follows:

- Versions are numbered according to semantic versioning: `vX.Y.Z`
- A new release is produced on a monthly basis
- Four releases a year are chosen for [long term support (LTS)](https://github.com/tektoncd/community/blob/main/releases.md#support-policy).
  All remaining releases are supported for approximately 1 month (until the next
  release is produced)
    - LTS releases take place in January, April, July and October every year
    - The first Tekton Operator LTS release will be **v0.63.0** in November 2022

Tekton Operator produces nightly builds, publicly available on
`gcr.io/tekton-nightly`. 

More details are available in the [Tekton Operator release documentation][tekton-releases-docs].

### Transition Process

Before release v0.63 Tekton Operator has worked on the basis of an undocumented
support period of four months, which will be maintained for the releases between
v0.60 and v0.62.

## Release Process

Tekton Operator releases are made of YAML manifests and container images.
Manifests are published to cloud object-storage as well as
[GitHub][tekton-operator-releases]. Container images are signed by
[Sigstore][sigstore] via [Tekton Chains][tekton-chains]; signatures can be
verified through the [public key][chains-public-key] hosted by the Tekton Chains
project.

Further documentation available:

- The Tekton Operator [release documents][tekton-releases-docs]
- The Tekton Operator [release process][tekton-releases-process]
- [Installing Tekton][tekton-installation]
- Standard for [release notes][release-notes-standards]

## Releases

### v0.63

- **Latest Release**: [v0.63.0][v0-62-0] (2022-11-25) ([docs][v0-63-0-docs])
- **Initial Release**: [v0.63.0][v0-63-0] (2022-11-25)
- **End of Life**: 2023-11-25
- **Patch Releases**: [v0.63.0][v0-63-0]

### v0.62

- **Latest Release**: [v0.62.0][v0-62-0] (2022-09-20) ([docs][v0-62-0-docs])
- **Initial Release**: [v0.62.0][v0-62-0] (2022-09-20)
- **End of Life**: 2023-01-19
- **Patch Releases**: [v0.62.0][v0-62-0]

### v0.61

- **Latest Release**: [v0.61.0][v0-61-0] (2022-08-25) ([docs][v0-61-0-docs])
- **Initial Release**: [v0.61.0][v0-61-0] (2022-08-25)
- **End of Life**: 2022-12-24
- **Patch Releases**: [v0.61.0][v0-61-0]

### v0.60

- **Latest Release**: [v0.60.1][v0-60-1] (2022-07-28) ([docs][v0-60-1-docs])
- **Initial Release**: [v0.60.0][v0-60-0] (2022-06-28)
- **End of Life**: 2022-10-27
- **Patch Releases**: [v0.60.0][v0-60-0], [v0.60.1][v0-60-1]

## End of Life Releases

Older releases are EOL and available on [GitHub][tekton-pipeline-releases].


[release-policy]: https://github.com/tektoncd/community/blob/main/releases.md
[sigstore]: https://sigstore.dev
[tekton-chains]: https://github.com/tektoncd/chains
[tekton-operator-releases]: https://github.com/tektoncd/operator/releases
[chains-public-key]: https://github.com/tektoncd/chains/blob/main/tekton.pub
[tekton-releases-docs]: docs/release/README.md
[tekton-releases-process]: tekton/README.md
[tekton-installation]: docs/install.md
[release-notes-standards]:
    https://github.com/tektoncd/community/blob/main/standards.md#release-notes

[v0-62-0]: https://github.com/tektoncd/operator/releases/tag/v0.62.2
[v0-61-0]: https://github.com/tektoncd/operator/releases/tag/v0.61.0
[v0-60-1]: https://github.com/tektoncd/operator/releases/tag/v0.60.1
[v0-60-0]: https://github.com/tektoncd/operator/releases/tag/v0.60.0

[v0-62-0-docs]: https://github.com/tektoncd/operator/tree/v0.62.2/docs#tekton-pipelines
[v0-61-0-docs]: https://github.com/tektoncd/operator/tree/v0.61.0/docs#tekton-pipelines
[v0-60-1-docs]: https://github.com/tektoncd/operator/tree/v0.60.1/docs#tekton-pipelines
