---
name: commit-message
description: This skill should be used when the user asks to "create a commit", "generate commit message", "commit changes", "make a commit", mentions "conventional commits", or discusses commit message formatting. Provides guided workflow for creating properly formatted commit messages with line length validation and required trailers.
version: 0.2.0
---

# Conventional Commit Message Creation

Create properly formatted conventional commit messages following project standards with line length validation and required trailers.

## Purpose

Generate commit messages that:

- Follow conventional commits format (`type(scope): description`)
- Use component names or GitHub issue numbers as scope
- Respect line length limits (50 for subject, 72 for body)
- Include required trailers (Signed-off-by, Assisted-by)
- Pass gitlint validation

## Quick Workflow

1. **Analyze changes**: Run git status and git diff to understand modifications
2. **Determine scope**: Use component name from changed files, or GitHub issue number if available
3. **Generate message**: Create conventional commit message with proper formatting
4. **Add trailers**: Include Signed-off-by and Assisted-by trailers
5. **Confirm with user**: Display message and wait for approval before committing

**CRITICAL**: Never commit without explicit user confirmation.

## Conventional Commit Format

### Structure

```text
<type>(<scope>): <description>

[optional body]

Signed-off-by: <name> <email>
Assisted-by: <model-name> (via Cursor)
```text

### Type Selection

Choose the appropriate commit type based on changes:

| Type | Description | Example |
| ------ | ------------- | --------- |
| `feat` | New features | `feat(tektonpipeline): add HA support` |
| `fix` | Bug fixes | `fix(reconciler): resolve race condition` |
| `docs` | Documentation | `docs(README): update installation steps` |
| `refactor` | Code refactoring | `refactor(installerset): simplify logic` |
| `test` | Test changes | `test(tektonchain): add unit tests` |
| `chore` | Maintenance | `chore(deps): update go dependencies` |
| `build` | Build system | `build(Makefile): add vendor target` |
| `ci` | CI/CD changes | `ci(github): add golangci-lint action` |
| `perf` | Performance | `perf(cache): optimize lookup speed` |
| `style` | Code style | `style(format): run fumpt formatter` |
| `revert` | Revert commit | `revert: undo breaking API change` |

For complete type reference, see `references/commit-types.md`.

### Scope Rules

#### Priority 1: Component from changed files

Analyze staged files to identify the primary component:

```bash
# Get list of staged files
git diff --cached --name-only
```text

| File pattern | Scope | Example commit |
| ------------ | ----- | -------------- |
| `pkg/reconciler/kubernetes/tektonpipeline/*` | `tektonpipeline` | `fix(tektonpipeline): resolve status update` |
| `pkg/reconciler/kubernetes/tektonchain/*` | `tektonchain` | `feat(tektonchain): add OCI support` |
| `pkg/reconciler/openshift/*` | component name | `fix(tektondashboard): fix route creation` |
| `pkg/reconciler/common/*` | `common` | `refactor(common): simplify installerset` |
| `pkg/apis/*` | `api` | `feat(api): add new CRD field` |
| `docs/*` | `docs` or filename | `docs(README): update steps` |
| `test/*` | component being tested | `test(tektonchain): add unit tests` |
| `cmd/*` | command name | `feat(operator): add new flag` |
| Root files | filename | `chore(Makefile): add target` |
| `AGENTS.md`, `CLAUDE.md` | `docs` | `docs(AGENTS.md): update conventions` |

#### Priority 2: GitHub issue number (optional)

If the work is tracked in a GitHub issue and the user provides one, it can be used as the scope:

```text
# Branch: fix-123-installerset-race
# Scope: #123 or component name
# Result: fix(installerset): resolve race condition

Fixes #123
```text

Add `Fixes #NNN` or `Closes #NNN` in the commit body (not the scope) — this is the standard GitHub convention for auto-closing issues.

#### Priority 3: Ask user

If changed files span multiple components or scope is unclear, ask the user which component is the primary focus.

## Line Length Requirements

### Subject Line

- **Target**: 50 characters maximum
- **Hard limit**: 72 characters (gitlint enforced)
- **Format**: `type(scope): description` counts toward limit
- **Tips**: Use present tense, no period at end

```text
# Good (44 chars)
feat(tektonpipeline): add HA support

# Too long - will fail gitlint
feat(tektonpipeline): add comprehensive high-availability support for pipeline controller
```text

### Body

- **Wrap at 72 characters per line**
- **Blank line** required between subject and body
- **Content**: Explain why, not what (code shows what)
- **Format**: Wrap manually or use heredoc in git commit

```text
feat(tektonpipeline): add HA support

Enable leader election in the pipeline reconciler to support
high-availability deployments. This prevents split-brain issues
when multiple operator replicas are running.

Signed-off-by: Developer Name <developer@example.com>
Assisted-by: Claude Sonnet 4.5 (via Cursor)
```text

## Required Trailers

### Signed-off-by

**Always include**: `Signed-off-by: <name> <email>`

This certifies the Developer Certificate of Origin (DCO) — required by tektoncd upstream.

**Detection priority order**:

1. Environment variables: `$GIT_AUTHOR_NAME` and `$GIT_AUTHOR_EMAIL`
2. Git config: `git config user.name` and `git config user.email`
3. If neither configured, ask user to provide details

```bash
# Check environment variables first
echo "$GIT_AUTHOR_NAME <$GIT_AUTHOR_EMAIL>"

# Fallback to git config
git config user.name
git config user.email
```text

For complete detection logic, see `references/trailer-detection.md`.

### Assisted-by

**Always include**: `Assisted-by: <model-name> (via Cursor)`

**Format examples**:

```text
Assisted-by: Claude Sonnet 4.5 (via Cursor)
Assisted-by: Claude Opus 4.5 (via Cursor)
```text

Use the actual model name (Claude Sonnet 4.5, Claude Opus 4.5, etc.).

## User Confirmation Requirement

**CRITICAL RULE**: Always ask for user confirmation before executing `git commit`.

### Confirmation Workflow

1. **Generate** the commit message following all rules above
2. **Display** the complete message to the user with separator
3. **Ask**: "Should I commit with this message? (y/n)"
4. **Wait** for user response
5. **Commit** only if user confirms (yes/y/affirmative)

### Example Interaction

```text
Generated commit message:
---
feat(tektonchain): add OCI bundle support

Enable storing Tekton Chains signatures in OCI registries. This
allows signing artifacts without requiring a separate storage
backend.

Signed-off-by: Developer Name <developer@example.com>
Assisted-by: Claude Sonnet 4.5 (via Cursor)
---

Should I commit with this message? (y/n)
```text

Wait for user response before proceeding.

## Commit Execution

Use heredoc format for proper multi-line handling:

```bash
git commit -m "$(cat <<'EOF'
feat(tektonchain): add OCI bundle support

Enable storing Tekton Chains signatures in OCI registries.

Signed-off-by: Developer Name <developer@example.com>
Assisted-by: Claude Sonnet 4.5 (via Cursor)
EOF
)"
```text

**Never use**:

- `--no-verify` (skips pre-commit hooks)
- `--no-gpg-sign` (skips signing)
- `--amend` (unless explicitly requested and safe)

## Complete Examples

### Feature with component scope

```text
feat(tektonpipeline): add HA leader election

Enable leader election in the pipeline reconciler to support
high-availability deployments with multiple operator replicas.

Signed-off-by: Jane Developer <jane@example.com>
Assisted-by: Claude Sonnet 4.5 (via Cursor)
```text

### Bug fix closing a GitHub issue

```text
fix(installerset): resolve concurrent reconcile panic

Prevent nil pointer dereference when two reconcile loops run
concurrently on the same InstallerSet object.

Fixes #789

Signed-off-by: John Developer <john@example.com>
Assisted-by: Claude Sonnet 4.5 (via Cursor)
```text

### Documentation update

```text
docs(AGENTS.md): update architecture conventions

Add InstallerSet naming patterns and clarify platform split
rules for contributors.

Signed-off-by: Jane Developer <jane@example.com>
Assisted-by: Claude Sonnet 4.5 (via Cursor)
```text

### Breaking change

```text
feat(api)!: remove deprecated TektonConfig fields

Remove fields deprecated in v0.60 from TektonConfig spec.
Operators upgrading from v0.60 must migrate before upgrading.

BREAKING CHANGE: Removed spec.targetNamespace and spec.profile
from TektonConfig. Use spec.config instead. See migration guide.

Signed-off-by: John Developer <john@example.com>
Assisted-by: Claude Sonnet 4.5 (via Cursor)
```text

## Gitlint Integration

This project uses gitlint to enforce commit message format. Ensure all commit messages pass gitlint validation.

**Common gitlint rules**:

- Conventional commit format required
- Subject line length limits (50 soft, 72 hard)
- Required trailers (Signed-off-by)
- No trailing whitespace
- Body line wrapping at 72 characters

For complete gitlint rules, see `references/gitlint-rules.md`.

## Auto-Detection Summary

When generating commit messages:

1. Run `git status` (without -uall flag)
2. Run `git diff` for staged and unstaged changes
3. Identify primary component from staged file paths
4. If scope unclear, ask user
5. If user mentions a GitHub issue number, add `Fixes #NNN` to body
6. Analyze staged files to determine commit type
7. Generate appropriate scope and description
8. Detect author info from environment variables or git config
9. Ensure subject line is ≤50 characters (max 72)
10. Wrap body text at 72 characters per line
11. Add required trailers (Signed-off-by and Assisted-by)
12. Format according to conventional commits standard
13. **Display message and ask for user confirmation**
14. Only commit after receiving confirmation

## Additional Resources

For detailed information:

- **`references/commit-types.md`** - Complete commit type reference with descriptions
- **`references/gitlint-rules.md`** - Gitlint validation rules and configuration
- **`references/trailer-detection.md`** - Author detection logic and priority order
