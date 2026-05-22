<!--
---
linkTitle: "Installation"
weight: 100
---
-->

# Installing Tekton Operator
To configure images from a custom registry, follow the [Air Gap Configuration](./AirGapImageConfiguration.md) guide.

1. Install operator
    ```
    $ kubectl apply -f https://infra.tekton.dev/tekton-releases/operator/latest/release.yaml
    ```
    **Note**: This will also install pipelines, triggers, chains, and dashboard
2. In case you want to install other components, use available [installation profiles](https://github.com/tektoncd/operator/tree/main/config/crs/kubernetes/config): `lite`
   , `all`, `basic`

   Where

   | Platform              | Profile | Installed Component                                  |
   |-----------------------|---------|------------------------------------------------------|
   | Kubernetes, OpenShift | lite    | Pipeline                                             |
   | Kubernetes, OpenShift | basic   | Pipeline, Trigger, Chains                            |
   | Kubernetes            | all     | Pipeline, Trigger, Chains, Dashboard                 |
   | OpenShift             | all     | Pipeline, Trigger, Chains, Pipelines as Code, Addons |

    
    To install pipelines, triggers, chains and dashboard (use profile 'all')
    ```
    $ kubectl apply -f https://raw.githubusercontent.com/tektoncd/operator/main/config/crs/kubernetes/config/all/operator_v1alpha1_config_cr.yaml
    ```

## Platform notes

### OpenShift: do not run pipelines in the `default` namespace

On OpenShift, the `default` namespace is classified as a "highly privileged" system namespace. Pod Security Admission (PSA) label synchronization is permanently disabled there by the platform, so even though the operator correctly creates the `pipeline` ServiceAccount and RBAC bindings in `default`, PipelineRuns submitted to that namespace fail with `permissionDenied` errors: PSA enforces the `restricted` profile and the SCC-to-PSA label sync never runs.

User-created namespaces are not affected because the Cluster Policy Controller automatically syncs SCC privileges into PSA labels. The OpenShift documentation has the same guidance ([Do not run workloads in or share access to default projects](https://docs.redhat.com/en/documentation/openshift_container_platform/4.18/html/building_applications/projects)).

Run pipelines in a dedicated namespace instead of `default` on OpenShift. See [tektoncd/operator#3427](https://github.com/tektoncd/operator/issues/3427) for the original report.
