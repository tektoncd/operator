<!--
---
title: "Operator"
linkTitle: "Operator"
weight: 2
description: >
  Manage Tekton CI/CD Building Blocks
cascade:
  github_project_repo: https://github.com/tektoncd/operator
---
-->
# Tekton Operator

Tekton Operator is a Kubernetes extension that to install, upgrade and
manage TektonCD [Pipelines](https://github.com/tektoncd/pipeline),
[Dashboard](https://github.com/tektoncd/dashboard),
[Triggers](https://github.com/tektoncd/triggers) (and other
components) on any Kubernetes Cluster.

## Tekton Operator entities

Tekton Operator defines the following entities:

<table>
  <tr>
    <th>Entity</th>
    <th>Description</th>
  </tr>
  <tr>
    <td><code>TektonConfig</code></td>
    <td>Configure Tekton components to be installed and managed.</td>
  </tr>
  <tr>
    <td><code>TektonPipeline</code></td>
    <td>Configure the <a HREF="https://github.com/tektoncd/pipeline">Tekton Pipeline</a> component to be installed and managed.</td>
  </tr>
  <tr>
    <td><code>TektonTrigger</code></td>
    <td>Configure the <a HREF="https://github.com/tektoncd/triggers">Tekton Trigger</a> component to be installed and managed.</td>
  </tr>
  <tr>
    <td><code>TektonDashboard</code></td>
    <td>Configure the <a HREF="https://github.com/tektoncd/dashboard">Tekton Dashboard</a> component to be installed and managed.</td>
  </tr>
  <tr>
    <td><code>TektonResult</code></td>
    <td>Configure the <a HREF="https://github.com/tektoncd/results">Tekton Result</a> component to be installed and managed.</td>
  </tr>
  <tr>
    <td><code>TektonAddon</code></td>
    <td>Configure addons to be installed and managed.</td>
  </tr>
</table>

## Getting started

TBD

## Understanding Tekton Operator

See the following topics to learn how to use Tekton Pipelines in your project:

- TBD

## Contributing to Tekton Operator

If you'd like to contribute to the Tekton Operator project, see the [Tekton Operator Contributor's Guide](https://github.com/tektoncd/operator/blob/main/CONTRIBUTING.md).

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
