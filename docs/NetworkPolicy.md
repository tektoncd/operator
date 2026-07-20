<!--
---
linkTitle: "NetworkPolicy"
weight: 15
---
-->
# NetworkPolicy

The operator can manage [NetworkPolicy][np] resources for Tekton component workloads.
Currently TektonPipeline (core controllers, resolvers, and proxy-webhook) and
TektonTrigger are supported; other components will be added later.

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

The `networkPolicy` field is propagated from `TektonConfig` to `TektonTrigger` and
`TektonPipeline`. Users can also configure it directly on the `TektonTrigger` or
`TektonPipeline` CR.

## Default Policies

When NetworkPolicy is enabled (the default), the following policies are applied
to the operand namespace (e.g. `tekton-pipelines` or `openshift-pipelines`):

### TektonPipeline

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `pipeline-default-deny` | deny all | — | All pods with `app.kubernetes.io/part-of: tekton-pipelines` |
| `pipeline-controller` | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 (K8s) or 5353 (OpenShift) | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| `pipeline-webhook` | ingress | TCP/8443 | Any (admission webhook) |
| | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| `pipeline-events-controller` | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| | egress | TCP/80, 443 | Any (CloudEvents sinks) |
| `pipeline-resolvers` | ingress | TCP/8080 | Pipeline controller pods |
| | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| | egress | TCP/80, 443 | Any (git HTTPS, OCI registries, Tekton Hub, http resolver) |
| | egress | TCP/22 | Any (git clone over SSH) |
| `tekton-proxy-webhook-default-deny` | deny all | — | All pods with `name: tekton-operator` (proxy-webhook) |
| `proxy-webhook` | ingress | TCP/8443 | Any (admission webhook) |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |

### TektonTrigger

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `tekton-default-deny` | deny all | — | All pods with `app.kubernetes.io/part-of: tekton-triggers` |
| `triggers-controller` | ingress | TCP/9000 | Prometheus namespace |
| | egress | UDP+TCP/53 (K8s) or 5353 (OpenShift) | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| `triggers-webhook` | ingress | TCP/8443 | Any (admission webhook) |
| | ingress | TCP/9000 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| `triggers-core-interceptors` | ingress | TCP/8443 | All namespaces (EventListeners) |
| | ingress | TCP/9000 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| | egress | TCP/80, 443 | Any (external APIs e.g. GitHub) |

### Console Plugin (OpenShift only)

The console plugin is a static file server (nginx) — all API calls run in the
user's browser via the OpenShift Console's proxy, not on this pod.

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `pipelines-console-plugin-deny` | deny all | — | All pods with `app: pipelines-console-plugin` |
| `pipelines-console-plugin` | ingress | TCP/8443 | `openshift-console` namespace |

These are static manifests shipped with the TektonConfig console plugin resources,
not reconciled via `spec.networkPolicy`.

All policies above are applied to the operand namespace (e.g. `tekton-pipelines`
or `openshift-pipelines`). They do not cover the operator's own namespace
(`tekton-operator` / `openshift-operators`), which ships fixed, non-configurable
NetworkPolicies as part of the operator's own install manifests/bundle (see
[Operator's own namespace](#operators-own-namespace) below).

### Platform differences

| Parameter | Kubernetes | OpenShift |
|---|---|---|
| DNS port | 53 | 5353 |
| DNS namespace | `kube-system` | `openshift-dns` |
| Prometheus namespace label | `kubernetes.io/metadata.name: monitoring` | `openshift.io/cluster-monitoring: "true"` |

## Operator's own namespace

The operator's own namespace (`tekton-operator` on Kubernetes, `openshift-operators`
on OpenShift) ships two fixed NetworkPolicies as static manifests alongside the
operator's Deployment/RBAC — in `config/kubernetes/base/networkpolicy.yaml` and
`config/openshift/base/networkpolicy.yaml` respectively. These are **not**
reconciled by a controller and are **not** configurable via `spec.networkPolicy`:
no CR watches the operator's own namespace, so there is nothing to gate this on.
They are also not a namespace-wide default-deny — each policy's `podSelector` is
scoped to one of the operator's own pods (`name: tekton-operator` /
`name: openshift-pipelines-operator` for the main controller, and
`name: tekton-operator-webhook` for the CR admission webhook) so that installing
the operator's bundle never affects unrelated pods that might share the namespace
(`openshift-operators` in particular is commonly shared by many operators).

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `tekton-operator` / `openshift-pipelines-operator` | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| `tekton-operator-webhook` | ingress | TCP/8443 | Any (admission webhook) |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |

**OpenShift caveat**: `openshift-operators` is a shared namespace where OLM installs
operators from OperatorHub, many of which ship no NetworkPolicy of their own. To
avoid silently breaking those operators' networking, OpenShift's platform payload
ships a permissive `default-allow-all` NetworkPolicy in that namespace out of the
box (labeled `capability.openshift.io/name: OperatorLifecycleManager`), with an
empty `podSelector` allowing all ingress/egress for every pod in the namespace.
Because NetworkPolicy rules are additive (a pod's allowed traffic is the union of
every policy that selects it, not the intersection), this platform-shipped policy
supersedes the two policies above in practice — the operator's own pods remain
fully open on a stock OpenShift cluster until a cluster admin removes or replaces
`default-allow-all`. The `openshift-pipelines` (operand) namespace has no such
baseline policy, so the `proxy-webhook` policies further up this page are enforced
as documented without this caveat.

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
