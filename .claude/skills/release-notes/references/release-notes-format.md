# Release Notes Format Reference

## Entry Format

Each release note entry MUST follow this exact format. The Link and Jira lines MUST be indented with two spaces so they render as nested sub-bullets:

```markdown
* **Bold title:** One-sentence description of the change.
  * Link: <PR_OR_COMMIT_URL>
  * Jira: [SRVKP-XXXX](https://issues.redhat.com/browse/SRVKP-XXXX)
```

### Rules

- The first bullet MUST start with `*` (no indent) with a bold title followed by a colon and a description.
- The Link line MUST start with `* Link:` (two-space indent) with the PR or commit URL.
- The Jira line MUST start with `* Jira:` (two-space indent). Include it ONLY if the entry has JIRA tickets. If there are multiple tickets, list each as a separate markdown link comma-separated.
- Within each section, list entries that have JIRA tickets FIRST, before entries without JIRA tickets.
- Do NOT add a Contributors section.

## Header Template

```markdown
# Tekton Operator {tag}

Tekton Operator {tag} has been released 🎉
```

## Section Headers

Use exactly these section headers (skip empty ones):

```markdown
## ✨ Major changes and Features
## 🐛 Bug Fixes
## 📚 Documentation Updates
## ⚙️ Chores
```

## Installation Section Template

```markdown
## Installation

To install this version apply the release manifest for your platform:

### Kubernetes

\`\`\`shell
kubectl apply -f https://github.com/{owner}/{repo}/releases/download/{tag}/release.yaml
\`\`\`

### OpenShift

\`\`\`shell
kubectl apply -f https://github.com/{owner}/{repo}/releases/download/{tag}/release.yaml
\`\`\`

### Documentation

https://github.com/{owner}/{repo}/tree/{tag}/docs
```

## JIRA Ticket Format

JIRA tickets matching `SRVKP-\d+` should be linked as:

```markdown
[SRVKP-XXXX](https://issues.redhat.com/browse/SRVKP-XXXX)
```

Multiple tickets are comma-separated:

```markdown
[SRVKP-1234](https://issues.redhat.com/browse/SRVKP-1234), [SRVKP-5678](https://issues.redhat.com/browse/SRVKP-5678)
```

## Complete Example

```markdown
# Tekton Operator v0.13.0

Tekton Operator v0.13.0 has been released 🎉

## ✨ Major changes and Features

* **Add TektonScheduler component support:** Introduced new TektonScheduler CRD for managing scheduler deployments via the operator.
  * Link: https://github.com/tektoncd/operator/pull/1234
  * Jira: [SRVKP-1234](https://issues.redhat.com/browse/SRVKP-1234)
* **Support StatefulSet ordinals for TektonPipeline:** Added opt-in StatefulSet ordinals mode for pipeline controller deployments.
  * Link: https://github.com/tektoncd/operator/pull/1230

## 🐛 Bug Fixes

* **Fix reconciler requeue on namespace mismatch:** Corrected requeue logic when target namespace changes between reconcile loops.
  * Link: https://github.com/tektoncd/operator/pull/1220
  * Jira: [SRVKP-5678](https://issues.redhat.com/browse/SRVKP-5678)

## ⚙️ Chores

* **Bump go.opentelemetry.io/otel from 1.28.0 to 1.29.0:** Updated OpenTelemetry dependency to latest version.
  * Link: https://github.com/tektoncd/operator/pull/1215

## Installation

To install this version apply the release manifest for your platform:

### Kubernetes

\`\`\`shell
kubectl apply -f https://github.com/tektoncd/operator/releases/download/v0.13.0/release.yaml
\`\`\`

### OpenShift

\`\`\`shell
kubectl apply -f https://github.com/tektoncd/operator/releases/download/v0.13.0/release.yaml
\`\`\`

### Documentation

https://github.com/tektoncd/operator/tree/v0.13.0/docs

## What's Changed
<!-- GitHub auto-generated changelog goes here -->
```
