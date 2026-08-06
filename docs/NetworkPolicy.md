<!--
---
linkTitle: "NetworkPolicy"
weight: 15
---
-->
# NetworkPolicy

The operator can manage [NetworkPolicy][np] resources for Tekton component workloads.
TektonPipeline (core controllers, resolvers, and proxy-webhook), TektonTrigger,
TektonChain, Pipelines-as-Code, ManualApprovalGate, TektonPruner, TektonResult,
and MultiCluster components (TektonScheduler, TektonMulticlusterProxyAAE,
SyncerService) are supported; other components will be added later.

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

The `networkPolicy` field is propagated from `TektonConfig` to `TektonPipeline`,
`TektonTrigger`, `TektonChain`, `TektonPruner`, `TektonResult`, Pipelines-as-Code,
and MultiCluster components. When those component CRs are managed by `TektonConfig`
(the usual install path), **TektonConfig is the source of truth**: edits to
`spec.networkPolicy` on the component CRs alone are overwritten on the next Config
reconcile. Configure NetworkPolicy via `TektonConfig.spec.networkPolicy`.

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

### TektonChain

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `chains-controller-default-deny` | deny all | — | All pods matching Chains controller selector |
| `chains-controller` | ingress | TCP/9090 | Prometheus namespace |
| | egress | all | Unrestricted (API server, OCI registries, Sigstore, KMS, storage backends — NP cannot select host-network endpoints) |

### OpenShift Pipelines-as-Code

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `pac-default-deny` | deny all | — | All pods with `app.kubernetes.io/part-of: pipelines-as-code` |
| `pac-controller` | ingress | TCP/9090 | Prometheus namespace |
| | ingress | TCP/8082 | Any (Git provider webhooks via Route/Ingress) |
| | egress | UDP+TCP/53 (K8s) or 5353 (OpenShift) | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| | egress | TCP/80, 443 | Any (Git provider APIs: GitHub, GitLab, Bitbucket, Gitea) |
| `pac-watcher` | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| | egress | TCP/80, 443 | Any (reporting status to Git providers) |
| `pac-webhook` | ingress | TCP/8443 | Any (admission webhook) |
| | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |

### ManualApprovalGate

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `mag-default-deny` | deny all | — | All pods with `app.kubernetes.io/part-of: openshift-pipelines-manual-approval-gates` |
| `mag-controller` | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 (K8s) or 5353 (OpenShift) | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| `mag-webhook` | ingress | TCP/8443 | Any (admission webhook) |
| | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |

The `networkPolicy` field is available directly on the `ManualApprovalGate` CR
(MAG is a standalone CR, not managed through `TektonConfig`):

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: ManualApprovalGate
metadata:
  name: manual-approval-gate
spec:
  networkPolicy:
    disabled: false
```

### TektonPruner

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `tekton-pruner-default-deny` | deny all | — | All pods with `app.kubernetes.io/part-of: tekton-pruner` |
| `pruner-controller` | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| `pruner-webhook` | ingress | TCP/8443 | Any (admission webhook) |
| | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |

### TektonResult

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `results-default-deny` | deny all | — | Pods with `app.kubernetes.io/name` in Results API, watcher, retention-policy-agent, postgres |
| `results-api` | ingress | TCP/8080 | All namespaces (Console Plugin, CLI, routes, watcher, internal clients) |
| | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 (K8s) or 5353 (OpenShift) | DNS resolver pods |
| | egress | TCP/`db_port` (default 5432) | Any destination (in-cluster or external DB; port from Results Spec) |
| | egress | all | API server (auth token review / impersonation) |
| `results-watcher` | ingress | TCP/9090 | Prometheus namespace |
| | egress | UDP+TCP/53 (K8s) or 5353 (OpenShift) | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints; also covers Results API) |
| `results-retention-policy-agent` | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | TCP/`db_port` (default 5432) | Any destination (in-cluster or external DB) |
| `results-postgres` | ingress | TCP/`db_port` | Results API and retention-policy-agent pods only |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |

API and retention DB egress is **port-only** (no pod/CIDR peer). NetworkPolicy cannot
match on hostname or JDBC URL, so restricting `To` to in-cluster postgres pods would
break external databases. Configure `db_host` / `db_port` as usual; when `db_port`
changes in the Results Spec, the operator regenerates these policies on reconcile.
No custom NetworkPolicy is required for an external database.

#### Administrative / debug access to in-cluster Postgres

`results-postgres` ingress allows only Results API and retention-policy-agent pods.
One-off workloads (DB migrations, `psql` debug pods) are otherwise dropped once
NetworkPolicy is enabled.

**Recommended:** add a dedicated temporary NetworkPolicy that allows a dedicated
admin label. Do **not** reuse `app: tekton-results-api` on a debug pod — that label
is also used by the Results API Service selector and can route API traffic to the
wrong pod. A dedicated label (below) avoids that.

Apply directly:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: results-postgres-admin
  namespace: tekton-pipelines   # or openshift-pipelines
spec:
  podSelector:
    matchLabels:
      app.kubernetes.io/name: tekton-results-postgres
  policyTypes: [Ingress]
  ingress:
    - from:
        - podSelector:
            matchLabels:
              app: tekton-results-db-admin
      ports:
        - protocol: TCP
          port: 5432   # match Spec db_port when customized
```

Or the same rule via `spec.networkPolicy.policies` (use a new name so you do not
replace a managed default):

```yaml
spec:
  networkPolicy:
    policies:
      results-postgres-admin:
        podSelector:
          matchLabels:
            app.kubernetes.io/name: tekton-results-postgres
        policyTypes: [Ingress]
        ingress:
          - from:
              - podSelector:
                  matchLabels:
                    app: tekton-results-db-admin
            ports:
              - protocol: TCP
                port: 5432
```

Label the migration/`psql` pod with `app: tekton-results-db-admin`, then remove the
policy (or the `policies` entry) when finished.

**Emergency only:** temporarily set `spec.networkPolicy.disabled: true` (on
`TektonConfig` when Config manages Results), run the admin work, then set
`disabled: false` again. Prefer the dedicated admin NetworkPolicy above so other
workloads stay locked down.

### Console Plugin (OpenShift only)

The console plugin is a static file server (nginx) — all API calls run in the
user's browser via the OpenShift Console's proxy, not on this pod.

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `pipelines-console-plugin-deny` | deny all | — | All pods with `app: pipelines-console-plugin` |
| `pipelines-console-plugin` | ingress | TCP/8443 | `openshift-console` namespace |

These are static manifests shipped with the TektonConfig console plugin resources,
not reconciled via `spec.networkPolicy`.

### TektonScheduler

Policies are applied to the operand namespace (`tekton-pipelines` or `openshift-pipelines`).

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `scheduler-controller-default-deny` | deny all | — | Scheduler controller pods |
| `scheduler-webhook-default-deny` | deny all | — | Scheduler webhook pods |
| `scheduler-controller` | ingress | TCP/8443 | Prometheus namespace |
| | egress | UDP+TCP/53 (K8s) or 5353 (OpenShift) | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |
| `scheduler-webhook` | ingress | TCP/9443 | Any (admission webhook) |
| | ingress | TCP/8443 | Prometheus namespace |
| | egress | UDP+TCP/53 or 5353 | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |

### TektonMulticlusterProxyAAE

Policies are applied to the operand namespace (`tekton-pipelines` or `openshift-pipelines`).
Deployed only when the scheduler is enabled with multi-cluster role = Hub.

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `proxy-aae-default-deny` | deny all | — | Proxy-AAE pods (`app: proxy-aae`) |
| `proxy-aae` | ingress | TCP/8080 | Any (spoke clusters connect via service 443→8080) |
| | egress | UDP+TCP/53 (K8s) or 5353 (OpenShift) | DNS resolver pods |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |

### SyncerService (OpenShift only)

Policies are applied to the operand namespace (`openshift-pipelines`).
Deployed only when the scheduler is enabled with multi-cluster role = Hub.

| Policy | Direction | Port | Source / Destination |
|---|---|---|---|
| `syncer-service-default-deny` | deny all | — | SyncerService pods (`app: workload-controller`) |
| `syncer-service-controller` | egress | UDP+TCP/5353 | DNS resolver pods (OpenShift) |
| | egress | all | API server (all egress allowed — NP cannot select host-network endpoints) |

All component policies (TektonPipeline, TektonTrigger, TektonScheduler,
TektonMulticlusterProxyAAE, SyncerService, and Console Plugin) are applied to the
operand namespace (e.g. `tekton-pipelines` or `openshift-pipelines`).

### Platform differences

| Parameter | Kubernetes | OpenShift |
|---|---|---|
| DNS port | 53 | 5353 |
| DNS namespace | `kube-system` | `openshift-dns` |
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
