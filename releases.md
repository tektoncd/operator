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
