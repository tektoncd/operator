# NamespaceSyncController Design

**Date:** 2026-06-26  
**Author:** Jawed Khelil  
**Related:** [RFE-7814](https://redhat.atlassian.net/browse/RFE-7814) · [SRVKP-11482](https://redhat.atlassian.net/browse/SRVKP-11482)  
**Status:** Draft

---

## Problem

When a namespace is created in OpenShift, the platform immediately creates default service
accounts (`default`, `builder`, `deployer`). The Quay Bridge Operator hooks into namespace
creation and binds Quay robot secrets to those SAs.

The `pipeline` SA — the default SA used by all Tekton PipelineRuns — is **not** an
OpenShift-default SA. It is created by the Tekton Operator's RBAC reconciler, which runs
after the namespace is ready. This means:

1. Quay Bridge has already run its binding pass before `pipeline` SA exists.
2. The RBAC reconciler stamps a `namespace-reconcile-version` label after its first pass and
   **never re-visits that namespace again** (until the next operator upgrade).
3. Any secret that arrives in a namespace after the first RBAC reconcile pass is silently
   missed — the `pipeline` SA never gets the binding, and PipelineRuns cannot push or pull
   images from Quay.

Users are currently forced to either manually bind secrets per namespace or grant elevated
SCCs to the `builder` SA, both of which introduce operational overhead and security risk at
scale.

---

## Goals

- Automatically bind configured secrets to the `pipeline` SA in every namespace.
- Handle both orderings: `pipeline` SA created before the secret, and secret created before
  `pipeline` SA.
- Handle secret rotation and deletion without manual intervention.
- No coupling to Quay Bridge Operator internals — work with any registry operator or secret
  source.
- Provide typed API fields (not stringly-typed `spec.params`) for cluster admins to declare
  binding intent.
- Replace the scan-based namespace reconciliation pattern with a watch-based reactive pattern
  that eliminates O(N) API calls per reconcile cycle.

## Non-Goals

- Changes to the Quay Bridge Operator codebase (out of scope for this operator).
- Automatic discovery of registry secrets without admin configuration.
- Managing image pull secrets for any SA other than `pipeline`.

---

## Background: Current RBAC Reconciler Limitations

The existing reconciler in `pkg/reconciler/openshift/tektonconfig/rbac.go` (1540 lines)
implements a **scan-based batch job** pattern:

```
TektonConfig reconciled
    └── rbac.Run()
            └── kubeClientSet.CoreV1().Namespaces().List()   ← direct API call, O(N)
                    └── for each namespace:
                            needsRBAC?    → processRBAC (SA, SCC, RoleBinding)
                            needsCABundle? → ensureCABundles
                            stamp namespace-reconcile-version label → skip forever
```

### Problems with this approach

| Problem | Impact |
|---|---|
| `List(all namespaces)` is a direct API server call on every TektonConfig reconcile | O(N) API calls; an existing `nsInformer` is available but unused for this path |
| Version label skip: namespaces are processed once per operator release | Late-arriving secrets never get bound until the next upgrade |
| All namespace concerns (SA, SCC, RoleBinding, CA bundles) in one 1540-line file | High coupling, hard to extend |
| `needsRBAC` self-healing check still calls `RoleBindings.Get()` per reconciled namespace | N additional API calls per TektonConfig reconcile in steady state |
| Namespace params (`createRbacResource`, `createCABundleConfigMaps`) are stringly-typed in `spec.params[]` | No type safety, no validation, hard to document |

### What the version label actually does

The label `openshift-pipelines.tekton.dev/namespace-reconcile-version: <VERSION>` serves two
purposes:

1. **Skip optimization** — avoid re-processing already-reconciled namespaces on each
   TektonConfig reconcile cycle.
2. **Upgrade propagation** — when `VERSION` changes on operator upgrade, the label no longer
   matches and all namespaces are re-reconciled, picking up any new SA or RBAC changes.

In a watch-based controller, both purposes are satisfied differently:
- Skip optimization: not needed — events only fire for the affected namespace.
- Upgrade propagation: not needed — operator pod restart (which always happens on upgrade)
  causes informers to perform a full `List`, re-enqueuing every namespace.

---

## Proposed Solution: `NamespaceSyncController`

A single new watch-based controller — `NamespaceSyncController` — inside the Tekton Operator
(OpenShift platform only). Its reconcile unit is the **namespace**. It owns all
namespace-level synchronisation concerns, replacing the scan-based pattern over time.

### Architecture

```
NamespaceSyncController
│
├── Watch: Namespace (create, delete)
│     → enqueue namespace name
│
├── Watch: ServiceAccount (field selector metadata.name=pipeline, create, delete)
│     → enqueue owning namespace
│
├── Watch: Secret (label selector from TektonConfig.spec.platforms.openshift.namespaceSync,
│                  create, delete, update)
│     → enqueue owning namespace
│
├── Watch: RoleBinding pipelines-scc-rolebinding (delete)
│     → enqueue owning namespace
│
└── Watch: ConfigMap config-trusted-cabundle / config-service-cabundle (delete)
      → enqueue owning namespace

ReconcileNamespace(ns string):
    if !config.namespaceSync.createPipelineSA      → skip
    ensurePipelineSA(ns)

    if !config.namespaceSync.createCABundles       → skip
    ensureCABundles(ns)

    ensureSCCRoleBinding(ns)

    if config.namespaceSync.createEditRoleBinding   → ensureEditRoleBinding(ns)
    else                                            → removeEditRoleBindingIfPresent(ns)

    for each binding in config.namespaceSync.secretBindings:
        ensureSecretBinding(ns, binding)            ← new, for this RFE
```

All event handlers enqueue the same thing — a namespace name — into a single work queue.
The reconciler receives a namespace name and ensures all desired state for that namespace
in one idempotent pass.

### Why one controller, not many

A single controller with multiple watches is the standard Kubernetes controller pattern and
avoids multiplying controller overhead (leader election, work queues, goroutines). Adding a
new sync concern means: add one watch + one `ensure*` call. The controller's scope is
well-defined: everything that must be true about a namespace for Tekton to function.

### Performance comparison

| | Current scan-based (rbac.go) | NamespaceSyncController |
|---|---|---|
| **Reads at startup** | None (lazy, on first TektonConfig reconcile) | 1× List per watched resource type (populates cache) |
| **Reads in steady state** | O(N) API calls per TektonConfig event | 0 API calls (served from informer cache) |
| **Reconcile scope per event** | All N namespaces | 1 namespace |
| **Memory overhead** | nsInformer already held | +Secret + SA filtered caches (~3–4 MB per 1000 ns) |
| **Upgrade propagation** | Version label comparison | Operator restart re-enqueues all namespaces |
| **Late-arriving resources** | Missed until next upgrade | Reactive: event fires, namespace reconciled immediately |

---

## Secret Binding Logic

### Handling late-arriving secrets

Two scenarios must work correctly:

**Scenario A — `pipeline` SA created before secret:**
1. Namespace created → `pipeline` SA created → `ensureSecretBinding` runs, finds no secret
   matching the configured selector → skips (no loop).
2. Later, secret is created in namespace → Secret watch fires → namespace enqueued →
   `ensureSecretBinding` finds secret, binds it to `pipeline` SA.

**Scenario B — Secret created before `pipeline` SA:**
1. Namespace created → secret created → SA watch fires for `pipeline` SA... but SA doesn't
   exist yet → `ensureSecretBinding` creates the SA then binds.  
   _Or_: SA watch fires when the `pipeline` SA is later created by `ensurePipelineSA` →
   namespace re-enqueued → binding completes.

**Secret deletion / rotation:**
- Secret deleted → Secret watch fires → namespace enqueued → `ensureSecretBinding` removes
  stale `imagePullSecrets` and `secrets` references from `pipeline` SA.
- Secret recreated (rotation) → Secret watch fires → namespace enqueued → new secret bound.

### Avoiding infinite reconcile loops

The binding check distinguishes two states:

| State | Action |
|---|---|
| Secret **does not exist** in this namespace | Skip — nothing to bind (namespace has no Quay integration) |
| Secret **exists** but **not bound** to `pipeline` SA | Reconcile — bind it |
| Secret **exists** and **already bound** | No-op — idempotent pass |

Only namespaces that have the secret but are missing the binding are ever re-reconciled.
Namespaces with no Quay integration (secret absent) are never looped on.

### Concurrent writes

The existing RBAC reconciler also writes to the `pipeline` SA (owner references). The
`NamespaceSyncController` must use `RetryOnConflict` when patching the SA to handle
concurrent updates without data loss.

---

## API Changes

### New typed field: `spec.platforms.openshift.namespaceSync`

`pkg/apis/operator/v1alpha1/openshift_platform.go`:

```go
type OpenShift struct {
    PipelinesAsCode        *PipelinesAsCode     `json:"pipelinesAsCode,omitempty"`
    SCC                    *SCC                 `json:"scc,omitempty"`
    EnableCentralTLSConfig *bool                `json:"enableCentralTLSConfig,omitempty"`
    // NamespaceSync controls namespace-level synchronisation performed by the
    // Tekton Operator in each user namespace on OpenShift.
    // +optional
    NamespaceSync *NamespaceSyncConfig `json:"namespaceSync,omitempty"`
}

// NamespaceSyncConfig configures what the NamespaceSyncController ensures in each namespace.
type NamespaceSyncConfig struct {
    // CreatePipelineSA controls whether the pipeline service account is created in each
    // namespace. Defaults to true.
    // +optional
    CreatePipelineSA *bool `json:"createPipelineSA,omitempty"`

    // CreateCABundles controls whether CA bundle ConfigMaps are injected into each namespace.
    // Defaults to true.
    // +optional
    CreateCABundles *bool `json:"createCABundles,omitempty"`

    // CreateEditRoleBinding controls whether a RoleBinding named openshift-pipelines-edit
    // is created in each namespace, binding the pipeline SA to the built-in edit ClusterRole.
    // This gives the pipeline SA broad write permissions within its namespace, which is
    // convenient for PipelineRuns that create or update resources. Set to false for
    // least-privilege environments.
    // Defaults to true (preserves historical behaviour).
    // Replaces the legacy spec.params entry legacyPipelineRbac.
    // +optional
    CreateEditRoleBinding *bool `json:"createEditRoleBinding,omitempty"`

    // SecretBindings declares secrets that should be bound to the pipeline SA in each
    // namespace. Each binding matches secrets by label selector or by name. When a matching
    // secret appears in a namespace, it is added to both imagePullSecrets and secrets on the
    // pipeline SA automatically.
    // +optional
    SecretBindings []SecretBinding `json:"secretBindings,omitempty"`
}

// SecretBinding describes a secret (or class of secrets) that should be bound to the
// pipeline SA. Exactly one of LabelSelector or SecretName must be set.
type SecretBinding struct {
    // LabelSelector selects secrets by label. All secrets matching this selector in a
    // given namespace are bound to the pipeline SA.
    // +optional
    LabelSelector *metav1.LabelSelector `json:"labelSelector,omitempty"`

    // SecretName binds a specific named secret in each namespace to the pipeline SA.
    // +optional
    SecretName string `json:"secretName,omitempty"`
}
```

### Example CR

```yaml
apiVersion: operator.tekton.dev/v1alpha1
kind: TektonConfig
metadata:
  name: config
spec:
  platforms:
    openshift:
      namespaceSync:
        createPipelineSA: true
        createCABundles: true
        createEditRoleBinding: true   # set to false for least-privilege environments
        secretBindings:
          # Bind any secret labeled with quay.io/robot-token=true (set by Quay Bridge Operator)
          - labelSelector:
              matchLabels:
                quay.io/robot-token: "true"
          # Or bind a specific named secret across all namespaces
          - secretName: my-registry-pullsecret
```

### Migration from `spec.params`

The existing `spec.params` entries (`createRbacResource`, `createCABundleConfigMaps`) are
deprecated in favour of the new typed fields. `SetDefaults` will read the old params, populate
the new typed fields, and emit a deprecation warning. The old params remain functional through
one release cycle before removal.

| Old `spec.params` entry | New typed field | Default |
|---|---|---|
| `createRbacResource: "false"` | `namespaceSync.createPipelineSA: false` | `true` |
| `createCABundleConfigMaps: "false"` | `namespaceSync.createCABundles: false` | `true` |
| `legacyPipelineRbac: "false"` | `namespaceSync.createEditRoleBinding: false` | `true` |

---

## Implementation Plan

### Phase 1 — API (no behaviour change)

1. Add `NamespaceSyncConfig` and `SecretBinding` types to `openshift_platform.go`.
2. Add `SetDefaults` migration for old `spec.params`.
3. Add `Validate` rules: at most one of `LabelSelector`/`SecretName` per binding; both empty
   is an error.
4. Run `./hack/update-codegen.sh` and commit generated files.

### Phase 2 — NamespaceSyncController (new file, does not touch rbac.go)

5. Create `pkg/reconciler/openshift/namespacesync/` package:
   - `controller.go` — registers informers and work queue.
   - `reconciler.go` — `ReconcileNamespace`: `ensurePipelineSA`, `ensureCABundles`,
     `ensureSCCRoleBinding`, `ensureSecretBindings`.
6. Move existing `ensureSA`, `ensureCABundles`, `ensureSCCRole` logic from `rbac.go` into
   the new reconciler (copy first, validate parity, then remove from `rbac.go` in Phase 3).
7. Register the new controller in `cmd/openshift/operator/main.go`.

### Phase 3 — Transition (parallel operation → rbac.go removal)

8. Run both controllers in parallel for one release cycle. `rbac.go` continues to stamp the
   version label; `NamespaceSyncController` handles events reactively.
9. Verify feature parity and confirm no regressions in E2E.
10. Remove `rbac.go` scan-based logic; retire the `namespace-reconcile-version` label.

### Phase 4 — Secret binding feature

11. Add `ensureSecretBindings` step in `ReconcileNamespace`.
12. Add filtered Secret informer in `controller.go` using the label selectors from
    `TektonConfig.Spec.Platforms.OpenShift.NamespaceSync.SecretBindings`.
13. Unit tests: SA-before-secret, secret-before-SA, secret rotation, deletion cleanup,
    no-secret namespace (no infinite loop), concurrent write conflict.
14. E2E test: namespace creation → Quay Bridge sync → `pipeline` SA binding verified →
    successful PipelineRun pushing to Quay.

---

## Open Questions

| # | Question | Status |
|---|---|---|
| 1 | Does Quay Bridge Operator label robot secrets with `quay.io/robot-token=true`? This label is required for the `labelSelector` binding to work without a secret name. | **Pending — confirm with Quay Bridge team once cluster is available** |
| 2 | Should the target SA name (`pipeline`) be hardcoded or made configurable via `NamespaceSyncConfig`? | Hardcoded for this iteration; can be added later. |
| 3 | What happens if a namespace has multiple matching secrets (e.g. one robot token per Quay org)? | All matching secrets are bound to `pipeline` SA. |
| 4 | Is there a cleanup requirement when OpenShift Pipelines is uninstalled? | `FinalizeKind` removes controller-created bindings from all `pipeline` SAs. |
| 5 | Should `secretBindings` also bind to `imagePullSecrets`, or only to `secrets`? | Both, matching the pattern used by Quay Bridge for `builder` SA. |

---

## Dependencies

- Quay Bridge Operator must label robot secrets with a stable label (e.g. `quay.io/robot-token=true`)
  for `labelSelector`-based bindings to work automatically. Without this, admins must use
  `secretName` bindings instead.
- The Tekton Operator's `SharedInformerFactory` must support filtered informer registration
  for Secrets and ServiceAccounts without breaking existing watchers.

---

## References

- [RFE-7814 — Flexible service account binding for Quay Robot credentials in OpenShift Pipelines](https://redhat.atlassian.net/browse/RFE-7814)
- [SRVKP-11482 — Automatic Quay Robot Secret Binding to Pipeline SA via Namespace-Scoped Controller](https://redhat.atlassian.net/browse/SRVKP-11482)
- `pkg/reconciler/openshift/tektonconfig/rbac.go` — existing RBAC reconciler (patterns to reuse)
- `pkg/apis/operator/v1alpha1/openshift_platform.go` — API types to extend
- `pkg/reconciler/openshift/tektonconfig/controller.go` — informer registration pattern
