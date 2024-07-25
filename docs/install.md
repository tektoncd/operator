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
    $ kubectl apply -f https://storage.googleapis.com/tekton-releases/operator/latest/release.yaml
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
