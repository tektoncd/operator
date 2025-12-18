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
`ghcr.io/tektoncd/operator`. 

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

## Releases

### In Support

| Version     | Minimum K8S | Pipeline    | Release Date | End of Life |
|-------------|-------------|-------------|--------------|-------------|
| v0.78.x LTS | 1.28.x      | v1.6.x LTS  | 2025-12-08   | 2026-12-08  |
| v0.77.x LTS | 1.28.x      | v1.3.1 LTS  | 2025-08-21   | 2026-08-21  |
| v0.76.x LTS | 1.28.x      | v1.0.0 LTS  | 2025-05-27   | 2026-05-27  |
| v0.75.x LTS | 1.28.x      | v0.68.x LTS | 2025-02-18   | 2026-02-18  |

### End of Life

| Version     | Minimum K8S | Pipeline    | Release Date | End of Life |
|-------------|-------------|-------------|--------------|-------------|
| v0.74.x LTS | 1.28.x      | v0.65.x     | 2024-11-22   | 2025-11-22  |
| v0.73.x     | 1.28.x      | v0.62.x     | 2024-10-01   | 2025-10-01  |
| v0.71.x     | 1.27.x      | v0.59.x     | 2024-06-06   | 2025-06-06  |
| v0.72.x     | 1.28.x      | v0.61.x     | 2024-07-11   | 2024-08-11  |
| v0.70.x LTS | 1.25.x      | v0.56.x LTS | 2024-02-21   | 2025-02-21  |
| v0.69.x LTS | 1.25.x      | v0.53.x LTS | 2023-12-28   | 2024-12-28  |
| v0.68.x LTS | 1.24.x      | v0.50.x     | 2023-09-22   | 2024-09-22  |
| v0.67.x LTS | 1.24.x      | v0.47.x LTS | 2023-05-25   | 2024-05-25  |
| v0.66.x     | 1.24.x      | v0.45.x     | 2023-03-20   | 2023-04-20  |
| v0.65.x LTS | 1.23.x      | v0.44.x LTS | 2023-03-09   | 2024-03-09  |
| v0.64.x     | 1.23.x      | v0.42.x     | 2023-01-04   | 2023-02-04  |
| v0.63.x LTS | 1.23.x      | v0.41.x LTS | 2022-11-25   | 2023-11-25  |
| v0.62.x     | 1.22.x      | v0.40.x     | 2022-09-20   | 2022-12-20  |
| v0.61.x     | 1.22.x      | v0.39.x     | 2022-08-25   | 2022-11-25  |
| v0.60.x     | 1.21.x      | v0.37.x     | 2022-06-28   | 2022-09-28  |
| v0.59.x     | 1.21.x      | v0.36.x     | 2022-06-03   | 2022-09-03  |
| v0.57.x     | 1.21.x      | v0.35.x     | 2022-04-29   | 2022-08-29  |
| v0.56.x     | 1.21.x      | v0.34.x     | 2022-04-19   | 2022-08-19  |
| v0.55.x     | 1.21.x      | v0.33.x     | 2022-03-14   | 2022-07-14  |
| v0.54.x     | 1.20.x      | v0.32.x     | 2022-01-17   | 2022-05-17  |
| v0.53.x     | 1.20.x      | v0.31.x     | 2021-12-09   | 2022-04-09  |
| v0.52.x     | 1.20.x      | v0.30.x     | 2021-11-26   | 2022-03-26  |
| v0.51.x     | 1.19.x      | v0.29.x     | 2021-11-03   | 2022-03-03  |
| v0.50.x     | 1.19.x      | v0.28.x     | 2021-10-19   | 2022-02-19  |

## Documentation References 

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
