<!--
---
linkTitle: "NetworkPolicy"
weight: 15
---
-->
# NetworkPolicy

The operator can manage [NetworkPolicy][np] resources for Tekton component workloads.
Currently only TektonTrigger is supported; other components will be added later.

Configuration is available via `TektonConfig`:

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  networkPolicy:
    disabled: false          # set to true to remove all managed NetworkPolicies
    policies:                # override or add policies by name
      triggers-controller:   # replaces the default triggers-controller policy
        podSelector:
          matchLabels:
            app: tekton-triggers-controller
        policyTypes: [Ingress]
        ingress:
          - ports:
              - port: 9000
```

The `networkPolicy` field is propagated from `TektonConfig` to `TektonTrigger`.
Users can also configure it directly on the `TektonTrigger` CR.

## Default Policies

When NetworkPolicy is enabled (the default), the following policies are applied
to the operand namespace (e.g. `tekton-pipelines` or `openshift-pipelines`):

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `tekton-default-deny` | deny all | — | All pods with `app.kubernetes.io/part-of: tekton-triggers` |
| `triggers-controller` | ingress | TCP/9000 | Prometheus namespace |
| | egress | UDP+TCP/53 (K8s) or 5353 (OpenShift) | DNS resolver pods |
| | egress | TCP/443 (K8s) or 6443 (OpenShift) | API server |
| `triggers-webhook` | ingress | TCP/8443 | Any (admission webhook) |
| | ingress | TCP/9000 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | TCP/443 or 6443 | API server |
| `triggers-core-interceptors` | ingress | TCP/8443 | All namespaces (EventListeners) |
| | ingress | TCP/9000 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | TCP/443 or 6443 | API server |
| | egress | TCP/80, 443 | Any (external APIs e.g. GitHub) |

### Platform differences

| Parameter | Kubernetes | OpenShift |
|---|---|---|
| DNS port | 53 | 5353 |
| DNS namespace | `kube-system` | `openshift-dns` |
| API server port | 443 | 6443 |
| Prometheus namespace label | `kubernetes.io/metadata.name: monitoring` | `openshift.io/cluster-monitoring: "true"` |

## Disabling

```yaml
spec:
  networkPolicy:
    disabled: true
```

This removes all managed NetworkPolicies from the operand namespace.

## Overriding a policy

Entries in `spec.networkPolicy.policies` replace or add policies by name.
To override a default policy, use the same name (e.g. `triggers-controller`).
New names add additional policies alongside the defaults.

[np]: https://kubernetes.io/docs/concepts/services-networking/network-policies/
