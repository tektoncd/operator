# Tekton Operator

Installs and manages Tekton components (Pipeline, Triggers, Chains, Dashboard,
Results, PAC…) on Kubernetes and OpenShift via custom CRDs and controllers.

---

## Build & Test Commands

```bash
# Build
make bin/kubernetes/operator
make bin/openshift/operator
make bin/kubernetes/webhook

# Test — runs without a cluster
make test
make test-unit
make test-unit-race 

# Lint — must pass before every PR
make lint # golangci-lint + yamllint (all packages)
make lint-go # Go only, all packages
make lint-go PKG=./pkg/reconciler/kubernetes/tektonchain/...  # single package (fast)

# Code generation — required after any API type change
./hack/update-codegen.sh

# Dependency update — required after go.mod changes
./hack/update-deps.sh
```

E2E tests require a live cluster and are tagged `//go:build e2e`.
See [DEVELOPMENT.md](./DEVELOPMENT.md) for cluster setup.

---

## Key Conventions

1. **Most components have both platform reconcilers, but not all.** `tektonaddon` is
   OpenShift-only; `tektondashboard` is Kubernetes-only. Before adding a new component,
   decide explicitly whether it is cross-platform or platform-specific.

2. **InstallerSet is the only way to apply manifests.** Never call `kubectl apply` or
   direct client writes inside a reconciler. Use the appropriate set type:
   - `MainSet` — all core resources. **Automatically splits** into two sets on the cluster:
     `<kind>-main-static-<rand>` (RBAC, CRDs, ConfigMaps) applied first, then
     `<kind>-main-deployment-<rand>` (Deployments + Services). `<kind>` = component name
     lowercased with `Tekton` prefix stripped (e.g. `TektonPipeline` → `pipeline`).
   - `PreSet` — resources that must exist *before* the main set (e.g. namespaces)
   - `PostSet` — resources applied *after* the main set is ready
   - `CustomSet("name", ...)` — independently versioned named set; produces
     `<kind>-<name>-<rand>` on the cluster (e.g. `chain-config-…`, `chain-secret-…`)

   Every InstallerSet carries labels: `operator.tekton.dev/created-by` (Go struct name,
   e.g. `TektonChain`), `operator.tekton.dev/type` (set type), and for main sub-sets
   `operator.tekton.dev/installType` (`static` | `deployment` | `statefulset`).

3. **One CR instance per component type.** `Validate()` enforces
   `GetName() != ComponentResourceName` — do not relax this check.

4. **Use `MarkXXX()` for status, never write `.Status.Conditions` directly.**
   All status transitions live in the generated `*_lifecycle.go` files.

5. **`v1alpha1.REQUEUE_EVENT_AFTER` means "retry later".** Return it for transient
   errors (e.g. dependency not yet ready). Return `nil` for permanent non-fatal states.

---

## Architecture (non-obvious parts)

**Built on Knative controller runtime.** Reconcilers implement `ReconcileKind` /
`FinalizeKind` from `knative.dev/pkg/reconciler`. Status conditions use
`knative.dev/pkg/apis` (`condSet.Manage(status).MarkTrue/MarkFalse`). Do not use
`controller-runtime` — it is not in this project.

**API layout** — all CRD types live in `pkg/apis/operator/`. Each component
requires four files:

```
tektonexample_types.go       # struct + compile-time interface assertions
tektonexample_defaults.go    # SetDefaults(ctx)
tektonexample_lifecycle.go   # MarkXXX status helpers
tektonexample_validation.go  # Validate(ctx)
```

The struct must carry these markers to drive `./hack/update-codegen.sh`:
```go
// +genclient
// +genreconciler:krshapedlogic=false
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +genclient:nonNamespaced
// +kubebuilder:resource:scope=Cluster
```

**Platform split** — shared behaviour lives in `pkg/reconciler/common/` and
`pkg/reconciler/shared/`; platform-specific overrides implement the `Extension`
interface (`PreReconcile`, `PostReconcile`, `Finalize`).

```
pkg/reconciler/
  kubernetes/<component>/   # Kubernetes-only reconciler
  openshift/<component>/    # OpenShift-only reconciler
  shared/<component>/       # reconcile logic used by both platforms
  common/                   # transformers, utilities, init-controller
```

**Init controller** — `pkg/reconciler/common/initcontroller.go` bootstraps manifests
at startup via `fetchSourceManifests`; every new component must be registered there.


---

## Pattern References for Common Changes

| Change | Canonical example to follow |
|--------|----------------------------|
| Reconciler structure | `pkg/reconciler/kubernetes/tektonpipeline/` |
| OpenShift extension | `pkg/reconciler/openshift/tektonpipeline/` |
| Manifest transformer | `pkg/reconciler/common/transformers.go` |
| API type + lifecycle | `pkg/apis/operator/v1alpha1/tektonpipeline_types.go` + `tektonpipeline_lifecycle.go` |
| Validation | `pkg/apis/operator/v1alpha1/tektonpipeline_validation.go` |
| E2E test helper | `test/resources/tektonpipeline.go` |

---

## PR Conventions

- Pull requests must follow the repository PR template defined in `.github/pull_request_template.md`.
- `make lint` must pass with zero issues.
- `make test` must pass with zero failures.
- Run `./hack/update-codegen.sh` and commit generated files whenever API types change.
- One CR instance per new component type — enforce in `Validate()`.

---

## Skills

For complex workflows, use these repo-local skills:

- **Commit messages**: Conventional commits with component scopes, line length validation, DCO Signed-off-by, and Assisted-by trailers. Trigger: "create commit", "commit changes", "generate commit message"
- **Release notes**: Gather PRs between tags, categorize, output formatted markdown, optionally update GitHub release. Trigger: "create release note", "generate release notes", "release changelog"


