<!--
---
linkTitle: "Operator Resources"
weight: 16
---
-->
# Operator Resource Requirements

The operator ships default CPU and memory requests and limits for its own
control-plane containers:

| Container                                  | requests    | limits      |
| ------------------------------------------ | ----------- | ----------- |
| operator lifecycle / cluster-operations  | 100m / 256Mi | 500m / 1Gi  |
| operator webhook                         | 100m / 100Mi | 500m / 500Mi |

The defaults are sized for small to medium installations. CPU usage of the
control plane stays very low even on large clusters, while memory scales
with the number of watched namespaces and pipelines activity.

Large installations should override the defaults.

## Kubernetes (Helm)

With the Helm chart, set `operator.resources` and `webhook.resources` in
your values file:

```yaml
operator:
  resources:
    requests:
      cpu: 250m
      memory: 512Mi
    limits:
      cpu: "1"
      memory: 2Gi
```

## OpenShift (OLM)

On OpenShift the operator is deployed by OLM from the OperatorHub
`Subscription`, so the resources are overridden at the subscription level.
OLM applies `spec.config.resources` to the operator deployment:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: openshift-pipelines-operator
  namespace: openshift-operators
spec:
  config:
    resources:
      requests:
        cpu: 250m
        memory: 512Mi
      limits:
        cpu: "1"
        memory: 2Gi
```
