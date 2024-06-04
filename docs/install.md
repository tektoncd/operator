<!--
---
linkTitle: "Installation"
weight: 100
---
-->

# Installing Tekton Operator

1. Install operator
    ```
    $ kubectl apply -f https://storage.googleapis.com/tekton-releases/operator/latest/release.yaml
    ```
    **Note**: This will also install pipelines, triggers, chains, and dashboard
2. In case you want to install other components, use available [installation profiles](https://github.com/tektoncd/operator/tree/main/config/crs/kubernetes/config): `lite`
   , `all`, `basic`

   Where

   | Profile | Installed Component | Platform |
   |---------|---------------------|----------|
   | lite | Pipeline | Kubernetes, Openshift |
   | basic | Pipeline, Trigger, Chains | Kubernetes, Openshift |
   | all | Pipeline, Trigger, Dashboard, Chains | Kubernetes |
   |  | Pipeline, Trigger, Addons, Pipelines as Code, Chains | Openshift |

    
     To install pipelines, triggers, chains and dashboard (use profile 'all')
    ```
    $ kubectl apply -f https://raw.githubusercontent.com/tektoncd/operator/main/config/crs/kubernetes/config/all/operator_v1alpha1_config_cr.yaml
    ```
