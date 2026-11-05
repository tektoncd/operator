# Assorted scripts for development

This directory contains several scripts useful in the development process of
Tekton Operator.

- [`update-codegen.sh`](./update-codegen.sh): Updates auto-generated client
  libraries.
- [`update-deps.sh`](./update-deps.sh): Updates Go dependencies.
- [`verify-codegen.sh`](./verify-codegen.sh): Verifies that auto-generated
  client libraries are up-to-date.

Payload fetching regression tests run with `go test ./hack`. They require
[mikefarah/yq v4](https://github.com/mikefarah/yq); downloads are mocked, and the
tests never modify the checkout or cluster.
