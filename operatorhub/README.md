# Generating Bundles (release artifact) for OperatorHub

We can use [bundle.py](./tools/bundle.py) tool to generate operator bundles. The tool takes in
a list of Kubernetes resource definitions (Deployment, Role, Rolebinding, ConfigMaps ...) and
some metadata (bundle description, release channels, bundle icon graphic... ) and outputs operator bundle.

## Quick Links

1. [Generating Operator Bundle for Kubernetes](./kubernetes/README.md)
1. [Generating Operator Bundle for OpenShift](./openshift/README.md)

## Concepts

### Bundle Generation "Strategies"

The bundle generation strategy specifies how the tool accumulates input (list of kubernetes resources).
Two strategies are supported:

1. **release-manifest**: input files are collected from a pre-existing release.yaml file. This release.yaml
   could be from an already existing github release or from the result of `ko resolve cofig`.

2. **local:** All input files are gathered in realtime from local repository extensive usage of `kustomize`.

### Update Strategies

An update strategy defines the update relations of the current bundle with the previous releases (bundles).
The bundle generation tool supports 2 update strategies.

1. **semver**: OLM will upgrade opertor to a bundle with higher semver version, in the current subscription channel
2. **replaces**: OLM will follow explict `spec.replaces: <previous version>` filed specification

more details: [semver-mode, replaces mode](https://k8s-operatorhub.github.io/community-operators/packaging-operator/)


## Conventions

The tool expects the following directory structure and a config.yaml for each platform.

```bash
kubernetes
├── config.yaml
├── manifests
│   ├── bases
│   │   └── tektoncd-operator.clusterserviceversion.template.yaml
│   ├── fetch-strategy-local
│   │   └── kustomization.yaml
│   └── fetch-strategy-release-manifest
│       └── kustomization.yaml
```

## CLI flags

  [bundle.py cli flags](./tools/bundle-too-cli-flags.md)

## Specifying Image Overrides using config.yaml

  [bunele generation config.yaml](./tools/CONFIG-DEFINITION.md)

## Operator LifeCycle Manager Concepts

- [Operator Bundle](https://operator-sdk.netlify.app/docs/olm-integration/quickstart-bundle/)
- [ClusterServiceVersion CRD](https://olm.operatorframework.io/docs/concepts/crds/clusterserviceversion/)

# References

2. [Operator Lifecycle Manager (OLM) Documentation](https://olm.operatorframework.io/docs/)
3. [Operator SDK OLM Integration Documentation](https://operator-sdk.netlify.app/docs/olm-integration/)
4. [OperatorHub (Community Operators) Documentation](https://k8s-operatorhub.github.io/community-operators/packaging-operator/)
