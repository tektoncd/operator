> [!IMPORTANT]
> **Migrate Images from *gcr.io* to *ghcr.io*.**
>
> To reduce costs, we've migrated all our new and old Tekton releases to the free tier on [ghcr.io/tektoncd](https://github.com/orgs/tektoncd/packages?repo_name=operator). <br />
> Read more [here](https://tekton.dev/blog/2025/04/03/migration-to-github-container-registry/).

---

# Tektoncd Operator

[![Go Report Card](https://goreportcard.com/badge/tektoncd/operator)](https://goreportcard.com/report/tektoncd/operator)
[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/6548/badge)](https://bestpractices.coreinfrastructure.org/projects/6548)

<p align="center">
<img width="250" height="325"  src="tekton-operator.png" alt="Operator Icon" title="Operator Icon"></img>
</p>

The quickest and easiest way to install, upgrade and manage TektonCD [Pipelines](https://github.com/tektoncd/pipeline),
[Dashboard](https://github.com/tektoncd/dashboard), [Triggers](https://github.com/tektoncd/triggers)
on any Kubernetes Cluster.

## Want to start using Pipelines

- [Installing Tekton Operator](docs/install.md)
- Take a look at our [roadmap](ROADMAP.md)

## Read the docs

- [Concepts and Guides](docs/README.md)
- [Development Guide](DEVELOPMENT.md)
- [Testing Guide](test/README.md)
- [How to make a TektonCD/Operator Release](tekton/README.md)
- [OperatorHub Bundles](operatorhub/README.md)

## Want to contribute

We are so excited to have you!

- See [CONTRIBUTING.md](CONTRIBUTING.md) for an overview of our processes
- See [DEVELOPMENT.md](DEVELOPMENT.md) for how to get started
- [Deep dive](./docs/developers/README.md) into demystifying the inner workings
  (advanced reading material)
- Look at our
  [good first issues](https://github.com/tektoncd/operator/issues?q=is%3Aissue+is%3Aopen+label%3A%22good+first+issue%22)
  and our
  [help wanted issues](https://github.com/tektoncd/operator/issues?q=is%3Aissue+is%3Aopen+label%3A%22help+wanted%22)

## Releases
- [Release Frequency](releases.md)

### In Support

| Version     | Minimum K8S | Pipeline    | Release Date | End of Life |
|-------------|-------------|-------------|--------------|-------------|
| v0.77.x LTS | 1.28.x      | v1.3.1 LTS  | 2025-08-21   | 2026-08-21  |
| v0.76.x LTS | 1.28.x      | v1.0.0 LTS  | 2025-05-27   | 2026-05-27  |
| v0.75.x LTS | 1.28.x      | v0.68.x LTS | 2025-02-18   | 2026-02-18  |
| v0.74.x LTS | 1.28.x      | v0.65.x     | 2024-11-22   | 2025-11-22  |
| v0.73.x     | 1.28.x      | v0.62.x     | 2024-10-01   | 2025-10-01  |


### End of Life

| Version     | Minimum K8S | Pipeline    | Release Date | End of Life |
|-------------|-------------|-------------|--------------|-------------|
| v0.71.x     | 1.27.x      | v0.59.x     | 2024-06-06   | 2025-06-06  |
| v0.72.x     | 1.28.x      | v0.61.x     | 2024-07-11   | 2024-08-11  |
| v0.70.x     | 1.25.x      | v0.56.x     | 2024-02-21   | 2025-02-21  |
| v0.69.x     | 1.25.x      | v0.53.x     | 2023-12-28   | 2024-12-28  |
| v0.68.x     | 1.24.x      | v0.50.x     | 2023-09-22   | 2024-09-22  |
| v0.67.x     | 1.24.x      | v0.47.x     | 2023-05-25   | 2024-05-25  |
| v0.66.x     | 1.24.x      | v0.45.x     | 2023-03-20   | 2023-04-20  |
| v0.65.x     | 1.23.x      | v0.44.x     | 2023-03-09   | 2024-03-09  |
| v0.64.x     | 1.23.x      | v0.42.x     | 2023-01-04   | 2023-02-04  |
| v0.63.x     | 1.23.x      | v0.41.x     | 2022-11-25   | 2023-11-25  |
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
