# Tekton Operator Helm Chart

This Helm chart installs the [Tekton Operator](https://tekton.dev/docs/operator/) into your Kubernetes (v1.16+) or Openshift cluster (v4.3+).
Once this chart is published it can be installed directly, until then [helm-git](https://github.com/aslafy-z/helm-git) can be used.

## TL;DR

Install [helm-git](https://github.com/aslafy-z/helm-git) plugin.

```sh
# Add the repo to helm (typically use a tag rather than main):
helm repo add tektoncd "git+https://github.com/tektoncd/operator@charts?ref=main"

# Install the operator
helm install tekton-operator \
  -n tekton-operator \
  --create-namespace \
  --set installCRDs=true \
  tektoncd/tekton-operator

# or install the Openshift flavor:
helm install tekton-operator \
  -n openshift-operators \
  --set openshift.enabled=true \
  --set installCRDs=true \
  tektoncd/tekton-operator
```

## Introduction

The Tekton operator is installed into the `tekton-operator` namespace for Kubernetes clusters and into `openshift-operators` for Openshift clusters (`openshift.enabled=true`).

The Tekton Custom Resource Definitions (CRDs) can either be installed manually (the recommended approach, see the [Tekton Operator Releases page](https://github.com/tektoncd/operator/releases)) or as part of the Helm chart (`installCRDs=true`).

**Important:** The Tekton operator components (especially the webhook) require the CRDs to be present during startup. If you set `installCRDs=false`, you **MUST** install the CRDs manually **BEFORE** installing the operator, otherwise the operator pods will fail to start with errors like "the server could not find the requested resource".

Installing the CRDs as part of the Helm chart(`installCRDs=true`) is not recommended for production setups, since uninstalling the Helm chart will also uninstall the CRDs and subsequently delete any remaining CRs.
The CRDs allow you to configure individual parts of your Tekton setup:

* [`TektonConfig`](https://tekton.dev/docs/operator/tektonconfig/)
* [`TektonPipeline`](https://tekton.dev/docs/operator/tektonpipeline/)
* [`TektonTrigger`](https://tekton.dev/docs/operator/tektontrigger/)
* [`TektonDashboard`](https://tekton.dev/docs/operator/tektondashboard/)
* [`TektonResult`](https://tekton.dev/docs/operator/tektonresult/)
* [`TektonAddon`](https://tekton.dev/docs/operator/tektonaddon/)
* [`TektonChain`](https://tekton.dev/docs/operator/tektonchain/)


After the installation of the Tekton-operater chart, you can start inject the Custom Resources (CRs) into your cluster.
The Tekton operator will then automatically start installing the components.
Please see the documentation of each CR for details.

## Uninstalling

Before removing the Tekton operator from your cluster, you should first make sure that there are no instances of resources managed by the operator left:

```sh
kubectl get TektonConfig,TektonPipeline,TektonDashboard,TektonInstallerSet,TektonResults,TektonTrigger,TektonAddon,TektonPruner,TektonChain --all-namespaces
```

Now you can use Helm to uninstall the Tekton operator:

```sh
# for Kubernetes:
helm uninstall --namespace tekton-operator tekton-operator --wait
kubectl delete namespace tekton-operator
# for Openshift:
helm uninstall --namespace openshift-operators --wait

# optionally remove repository from helm:
helm repo remove tektoncd
```

**Important:** if you installed the CRDs with the Helm chart (by setting `installCRDs=true`), the CRDs will be removed as well: this means any remaining Tekton resources (e.g. Tekton Pipelines) in the cluster will be deleted!

If you installed the CRDs manually, you can use the following command to remove them (*this will remove all Tekton resources from your cluster*):
```
kubectl delete crd TektonConfig TektonPipeline TektonDashboard TektonInstallerSet TektonResults TektonTrigger TektonAddon --ignore-not-found
```
