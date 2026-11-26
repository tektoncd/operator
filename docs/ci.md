# CI workflows

Overview of GitHub Actions under [`.github/workflows/`](../.github/workflows/).
Use this when a PR check fails, a nightly job flakes, or you need to re-run something on demand.

Slash commands require **write** access to the repository. They must be the first
token on the first line of a PR comment (see
[slash-command-dispatch](https://github.com/peter-evans/slash-command-dispatch)).

## Force / recover (quick reference)

| Goal | How |
|------|-----|
| Re-run **failed** Actions on a PR | Comment `/retest` |
| Run upgrade e2e against a PR head | Comment `/test upgrade` |
| Cherry-pick a **merged** PR | Comment `/cherry-pick <branch> [branch…]` |
| Re-run a specific workflow run | Actions UI → failed run → **Re-run failed jobs** / **Re-run all jobs** |
| Start a workflow that supports it | Actions → workflow → **Run workflow** (`workflow_dispatch`) |

`/retest` is implemented by [`slash.yml`](../.github/workflows/slash.yml) →
[`chatops_retest.yaml`](../.github/workflows/chatops_retest.yaml) (plumbing
reusable workflow). It re-runs failed checks; it does not invent new jobs.

## PR and push checks

These run on pull requests and/or pushes to `main` (unless noted).

| Workflow | File | Function | When it runs | Force run |
|----------|------|----------|--------------|-----------|
| **ci** | [`ci.yaml`](../.github/workflows/ci.yaml) | Path-filtered gate: build, lint (gofmt, golangci-lint, yamllint, licenses), unit+race tests, codegen verify, multi-arch `ko resolve`, kind e2e (via e2e-matrix), helm lint when charts change; **CI summary** aggregates results | `pull_request`, `push` to `main` | `/retest` or Actions re-run. Docs-only PRs skip most jobs (`non-docs` filter). |
| **Tekton Integration** | [`e2e-matrix.yml`](../.github/workflows/e2e-matrix.yml) | Kind e2e matrix (`k8s-oldest` / `k8s-latest`) | Called from **ci** only (`workflow_call`) | Re-run the **ci** workflow (or `/retest`). Not runnable alone from the UI. |
| **Helm Chart** | [`helm-chart.yaml`](../.github/workflows/helm-chart.yaml) | `helm lint` for charts | Called from **ci** when `charts/` or this workflow file changes | Same as above. |
| **Go coverage** | [`go-coverage.yml`](../.github/workflows/go-coverage.yml) | Unit coverage + Codecov / PR comment | `pull_request`/`push` to `main`; schedule every 2 months; `workflow_dispatch` | Actions → **Go coverage** → Run workflow, or re-run the check. |
| **CodeQL** | [`codeql-analysis.yml`](../.github/workflows/codeql-analysis.yml) | Static security analysis | `push`/`pull_request` to `main`; weekly Tue 20:30 UTC | Actions re-run (no slash shortcut). |
| **zizmor** | [`zizmor.yaml`](../.github/workflows/zizmor.yaml) | GitHub Actions security lint | `push` to `main`, all `pull_request`s | Actions re-run. |

### What **ci** runs (jobs)

After change detection:

1. **build** — `go build ./…` (skipped if only `.md` changed)
2. **lint** — gofmt, golangci-lint (new-from-merge-base), yamllint (if YAML changed), license check
3. **test** — `make test-unit-verbose-and-race`
4. **Check generated code** — `./hack/verify-codegen.sh`
5. **Multi-arch build** — `ko resolve` for kubernetes and openshift
6. **e2e-tests** — reusable e2e-matrix (kind)
7. **helm-chart** — only if chart paths changed
8. **CI summary** — fails if any required job failed (skipped counts as OK)

## On-demand and chatops

| Workflow | File | Function | When it runs | Force run |
|----------|------|----------|--------------|-----------|
| **Slash Command Routing** | [`slash.yml`](../.github/workflows/slash.yml) | Routes PR comments to other workflows | `issue_comment` created | N/A (trigger by commenting) |
| **Rerun Failed Actions** | [`chatops_retest.yaml`](../.github/workflows/chatops_retest.yaml) | Re-runs failed PR checks | `repository_dispatch` `retest-command` | `/retest` |
| **Cherry Pick Command** | [`cherry-pick-command.yaml`](../.github/workflows/cherry-pick-command.yaml) | Cherry-picks a merged PR onto release branch(es) | `repository_dispatch` `cherry-pick-command` | `/cherry-pick release-vX.Y.x` |
| **Upgrade tests** | [`upgrade-tests.yml`](../.github/workflows/upgrade-tests.yml) | Latest release → workloads → upgrade to HEAD → assert | Nightly 02:00 UTC; `workflow_dispatch`; `/test upgrade` on a PR | Comment `/test upgrade`, or Actions → **Upgrade tests** → Run workflow. **Does not run on every PR.** See also [test/README.md](../test/README.md). |

## Scheduled maintenance / automation

These open PRs or trigger release plumbing. Prefer **Run workflow** when stuck; do not use `/retest` for them.

| Workflow | File | Function | Schedule | Force run |
|----------|------|----------|----------|-----------|
| **Bump components.yaml on main** | [`bump-payload-on-main.yaml`](../.github/workflows/bump-payload-on-main.yaml) | Bump component payload versions on `main`, open PR | Weekdays 01:00 UTC | Actions → Run workflow |
| **Bump components.yaml on release branches** | [`bump-payload-on-releases.yaml`](../.github/workflows/bump-payload-on-releases.yaml) | Same for release branches (bugfix-oriented bump) | Weekdays 02:00 UTC | Actions → Run workflow |
| **Update third-party image digests** | [`update-third-party-image-digests.yaml`](../.github/workflows/update-third-party-image-digests.yaml) | Refresh digests, open PR | Wed 03:00 UTC | Actions → Run workflow |
| **Update Tekton Task Versions** | [`update-tektoncd-task-versions.yml`](../.github/workflows/update-tektoncd-task-versions.yml) | Sync task versions from catalog, open PR | Daily 00:00 UTC | Actions → Run workflow |
| **Regenerate Dependabot Configuration** | [`dependabot-regen.yml`](../.github/workflows/dependabot-regen.yml) | Regenerate `.github/dependabot.yml`, open PR | Mondays 09:00 UTC | Actions → Run workflow |
| **Patch Release** | [`patch-release.yaml`](../.github/workflows/patch-release.yaml) | Scan release branches / trigger PAC patch release | Thu 10:00 UTC; also manual with `branch` + `version` | Actions → **Patch Release** → Run workflow (inputs required for manual) |

## Release publishing

| Workflow | File | Function | When it runs | Force run |
|----------|------|----------|--------------|-----------|
| **Release Charts** | [`helm-release.yaml`](../.github/workflows/helm-release.yaml) | Publish Helm chart for tag `v*` | Push of tags matching `v*` | Push/retag the release tag (or re-run that tag’s workflow run) |

For operator release process itself, see [docs/release/README.md](release/README.md).

## Tips when investigating failures

- Prefer the **CI summary** job on **ci** to see which child failed vs skipped.
- Docs-only PRs often show many **skipped** jobs; that is expected.
- Upgrade failures on a PR almost always mean you need `/test upgrade` (or the nightly), not the default kind e2e matrix.
- Local equivalents: `make lint`, `make test-unit-race`, `./hack/verify-codegen.sh`, and [test/README.md](../test/README.md) for e2e/upgrade.
