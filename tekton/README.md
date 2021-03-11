# Tekton Operator Repo CI/CD

_Why does Tekton Operator have a folder called `tekton`? Cuz we think it would be cool
if the `tekton` folder were the place to look for CI/CD logic in most repos!_

We dogfood our projects by using Tekton Pipelines to build, test and release
Tekton Projects! This directory contains the
[`Tasks`](https://github.com/tektoncd/pipeline/blob/master/docs/tasks.md) and
[`Pipelines`](https://github.com/tektoncd/pipeline/blob/master/docs/pipelines.md)
that we use for Tekton Operator buind, test and release.

* [How to create a release](#create-an-official-release)
* [How to create a patch release](#create-a-patch-release)
* [Automated nightly releases](#nightly-releases)
* [Setup releases](#setup)

## Create an official release

To create an official release, follow the steps in the [release-cheat-sheet](./release-cheat-sheet.md).

## Nightly releases

[The nightly release pipeline](overlays/nightly-releases/operator-nightly-release-pipeline.yaml) is
[triggered nightly by Tekton](https://github.com/tektoncd/plumbing/tree/master/tekton).

## Setup from scratch

To start from scratch and use these Pipelines and Tasks: follow steps in [setup-release-from-scratch](./setup-release-from-scratch.md).
