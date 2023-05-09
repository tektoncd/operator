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
2. Install Components (
   uses [installation profiles](https://github.com/tektoncd/operator/tree/main/config/crs/kubernetes/config): `lite`
   , `all`, `basic`)

   Where

   | Profile | Installed Component | Platform |
   |---------|---------------------|----------|
   | lite | Pipeline | Kubernetes, Openshift |
   | basic | Pipeline, Trigger | Kubernetes, Openshift |
   | all | Pipeline, Trigger, Dashboard | Kubernetes |
   |  | Pipeline, Trigger, Addons, Pipelines as Code | Openshift |

    ```
    # to install pipelines, triggers and dashboard (use profile 'all')
    $ kubectl apply -f https://raw.githubusercontent.com/tektoncd/operator/main/config/crs/kubernetes/config/all/operator_v1alpha1_config_cr.yaml
    ```
