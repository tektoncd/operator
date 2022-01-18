# Tektoncd Operator

The quickest and easiest way to install, upgrade and manage TektonCD [Pipelines](https://github.com/tektoncd/pipeline),
[Dashboard](https://github.com/tektoncd/dashboard), [Triggers](https://github.com/tektoncd/triggers)
on any Kubernetes Cluster.

# Quick Start

## Install Tektoncd Operator

1. Install operator
    ```
    $ kubectl apply -f https://storage.googleapis.com/tekton-releases/operator/latest/release.yaml
    ```
2. Install Components (
   uses [installation profiles](https://github.com/tektoncd/operator/tree/main/config/crs/kubernetes/config): `lite`
   , `all`, `basic`)

   Where

   | Profile | Installed Component | Platform |
   |---------|---------------------|----------|
   | lite | Pipeline | Kubernetes, Openshift |
   | basic | Pipeline, Trigger | Kubernetes, Openshift |
   | all | Pipeline, Trigger, Dashboard | Kubernetes |
   |  | Pipeline, Trigger, Addons | Openshift |

    ```
    # to install pipelines, triggers and dashboard (use profile 'all')
    $ kubectl apply -f https://raw.githubusercontent.com/tektoncd/operator/main/config/crs/kubernetes/config/all/operator_v1alpha1_config_cr.yaml
    ```

# Detailed Documentation

[Concepts and Guides](docs/README.md)

# Development Guide

[Development Guide](docs/README.md)

# Running E2E tests

[Testing Guide](test/README.md)

# Release Guide

[How to make a TektonCD/Operator Release](tekton/README.md)

# Gerating OperatorHub Bundle(s)

[OperatorHub Bundles](operatorhub/README.md)

# Roadmap

[Roadmap](./ROADMAP.md)

# Read the docs

| Version                                                                  | Docs                                                                         |
|--------------------------------------------------------------------------|------------------------------------------------------------------------------|
| [HEAD](/README.md)                                                       | [Docs @ HEAD](/docs/README.md)                                               |
| [v0.54.0](https://github.com/tektoncd/operator/releases/tag/v0.54.0)     | [Docs @ v0.54.0](https://github.com/tektoncd/operator/tree/v0.54.0/docs)     | [Examples @ v0.22.0](https://github.com/tektoncd/pipeline/tree/v0.54.0/examples#examples) |
| [v0.23.0-2](https://github.com/tektoncd/operator/releases/tag/v0.23.0-2) | [Docs @ v0.23.0-2](https://github.com/tektoncd/operator/tree/v0.23.0-2/docs) | [Examples @ v0.22.0](https://github.com/tektoncd/pipeline/tree/v0.23.0-2/examples#examples) |
| [v0.23.0-1](https://github.com/tektoncd/operator/releases/tag/v0.23.0-1) | [Docs @ v0.23.0-1](https://github.com/tektoncd/operator/tree/v0.23.0-1/docs) | [Examples @ v0.22.0](https://github.com/tektoncd/pipeline/tree/v0.23.0-1/examples#examples) |
| [v0.22.0-3](https://github.com/tektoncd/operator/releases/tag/v0.22.0-3) | [Docs @ v0.22.0-3](https://github.com/tektoncd/operator/tree/v0.22.0-3/docs) | [Examples @ v0.22.0](https://github.com/tektoncd/pipeline/tree/v0.22.0-3/examples#examples) |
| [v0.22.0-2](https://github.com/tektoncd/operator/releases/tag/v0.22.0-2) | [Docs @ v0.22.0-2](https://github.com/tektoncd/operator/tree/v0.22.0-2/docs) | [Examples @ v0.22.0](https://github.com/tektoncd/pipeline/tree/v0.22.0-2/examples#examples) |
| [v0.22.0-1](https://github.com/tektoncd/operator/releases/tag/v0.22.0-1) | [Docs @ v0.22.0-1](https://github.com/tektoncd/operator/tree/v0.22.0-1/docs) | [Examples @ v0.22.0](https://github.com/tektoncd/pipeline/tree/v0.22.0-1/examples#examples) |
| [v0.21.0-1](https://github.com/tektoncd/operator/releases/tag/v0.21.0-1) | [Docs @ v0.21.0-1](https://github.com/tektoncd/operator/tree/v0.21.0-1/docs) | [Examples @ v0.21.0](https://github.com/tektoncd/pipeline/tree/v0.21.0-1/examples#examples) |
