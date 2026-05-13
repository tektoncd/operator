# Gitlint Integration and Rules

Complete reference for gitlint validation rules enforced in this project.

## Overview

Gitlint is a commit message linter that enforces conventional commits format and other quality standards. All commits must pass gitlint validation before being accepted.

## Installation and Usage

### Running Gitlint Locally

```bash
# Install gitlint (if not installed)
pip install gitlint

# Lint last commit
gitlint

# Lint specific commit
gitlint --commit abc123

# Lint commit message from stdin
echo "feat: add new feature" | gitlint

# Lint range of commits
gitlint --commits "origin/main..HEAD"
```text

### Pre-commit Integration

Gitlint runs automatically via pre-commit hooks:

```bash
# Install pre-commit hooks
pre-commit install

# Gitlint runs on git push
git push

# Skip gitlint (not recommended)
git push --no-verify

# Skip specific hook
SKIP=gitlint git push
```text

## Core Gitlint Rules

### T1: Title Max Length

**Rule**: Subject line must not exceed 72 characters (hard limit)
**Recommendation**: Target 50 characters for better readability

**Examples**:

✓ **Valid**:

```text
feat(SRVKP-123): add webhook controller
```text

(45 characters)

✗ **Invalid**:

```text
feat(SRVKP-123): add comprehensive webhook controller with GitHub integration support
```text

(78 characters, exceeds limit)

**How to fix**:

- Use concise, present-tense descriptions
- Move details to commit body
- Remove unnecessary words

✓ **Fixed**:

```text
feat(SRVKP-123): add webhook controller

Implements comprehensive GitHub integration with support for
push events, pull requests, and issue comments.
```text

### T2: Title Trailing Whitespace

**Rule**: Subject line must not have trailing whitespace

**Examples**:

✗ **Invalid**:

```text
feat(webhook): add handler
```text

(trailing spaces)

✓ **Valid**:

```text
feat(webhook): add handler
```text

**How to fix**: Remove trailing whitespace from subject line

### T3: Title Trailing Punctuation

**Rule**: Subject line must not end with punctuation (`.`, `!`, `?`, `,`, `:`, `;`)

**Exception**: `!` is allowed for breaking changes (e.g., `feat!:`)

**Examples**:

✗ **Invalid**:

```text
feat(webhook): add handler.
fix(controller): resolve bug!
```text

✓ **Valid**:

```text
feat(webhook): add handler
fix(controller): resolve bug
feat(api)!: change endpoint format
```text

**How to fix**: Remove ending punctuation from subject line

### T5: Body Line Length

**Rule**: Each line in commit body must not exceed 72 characters

**Examples**:

✗ **Invalid**:

```text
feat(webhook): add handler

This commit implements a comprehensive webhook handler for GitHub push events with support for validation.
```text

(Line 2 is 119 characters)

✓ **Valid**:

```text
feat(webhook): add handler

Implement comprehensive webhook handler for GitHub push events.
Includes validation and error handling.
```text

(Each line ≤ 72 characters)

**How to fix**: Wrap body text at 72 characters per line

### T6: Body Missing Blank Line

**Rule**: Must have blank line between subject and body

**Examples**:

✗ **Invalid**:

```text
feat(webhook): add handler
This implements webhook support.
```text

✓ **Valid**:

```text
feat(webhook): add handler

This implements webhook support.
```text

**How to fix**: Add blank line after subject

### T7: Body Match Regex

**Rule**: Enforces conventional commits format in subject line

**Pattern**: `^(feat|fix|docs|style|refactor|test|chore|build|ci|perf|revert)(\(.+\))?: .+`

**Requirements**:

- Start with valid type
- Optional scope in parentheses
- Colon and space after type/scope
- Non-empty description

**Examples**:

✓ **Valid**:

```text
feat(webhook): add handler
fix: resolve bug
docs(README): update steps
```text

✗ **Invalid**:

```text
Add handler                    (no type)
feat add handler              (missing colon)
feat: add handler.            (trailing period - T3 violation)
feature(webhook): add handler (invalid type)
```text

**How to fix**: Follow conventional commits format exactly

## Additional Rules

### B1: Body Max Line Length

**Rule**: Enforces 72-character limit for all body lines

Same as T5, but applies to entire body.

### B3: Body First Line Empty

**Rule**: First line after subject must be blank

Same as T6.

## Conventional Commits Validation

### Valid Type Enforcement

Gitlint enforces specific commit types:

- `feat` - Features
- `fix` - Bug fixes
- `docs` - Documentation
- `style` - Formatting
- `refactor` - Code refactoring
- `test` - Tests
- `chore` - Maintenance
- `build` - Build system
- `ci` - CI/CD
- `perf` - Performance
- `revert` - Reverts

**Invalid types will fail validation**:

```text
feature(webhook): add handler  # Use 'feat', not 'feature'
bugfix(api): resolve issue     # Use 'fix', not 'bugfix'
documentation: update README   # Use 'docs', not 'documentation'
```text

### Scope Format

**Optional but if present**:

- Must be in parentheses
- Must be after type
- Must be followed by colon

✓ **Valid**:

```text
feat(webhook): add handler     (with scope)
feat: add handler              (without scope)
```text

✗ **Invalid**:

```text
feat webhook: add handler      (missing parentheses)
(webhook)feat: add handler     (wrong position)
feat(webhook) add handler      (missing colon)
```text

## Trailer Validation

### Required Trailers

**Signed-off-by**: Required in this project

**Format**: `Signed-off-by: Name <email>`

**Examples**:

✓ **Valid**:

```text
Signed-off-by: John Developer <john@redhat.com>
```text

✗ **Invalid**:

```text
Signed-off-by: John Developer          (missing email)
Signed-off-by: john@redhat.com         (missing name)
signed-off-by: John Developer <john@>  (lowercase, invalid email)
```text

### Recommended Trailers

**Assisted-by**: Recommended when AI assists

**Format**: `Assisted-by: Model Name (via Tool)`

**Examples**:

✓ **Valid**:

```text
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
Assisted-by: Claude Opus 4.5 (via Claude Code)
```text

## Breaking Changes

### Indicating Breaking Changes

**Method 1**: Add `!` after type/scope:

```text
feat(api)!: change endpoint format
fix(controller)!: remove deprecated mode
```text

**Method 2**: Include `BREAKING CHANGE:` in footer:

```text
feat(api): update webhook payload

BREAKING CHANGE: Webhook payload structure changed. See migration
guide for update instructions.
```text

**Both methods can be combined for maximum clarity**.

## Common Gitlint Errors and Fixes

### Error: "Title exceeds max length (72>72)"

**Cause**: Subject line too long

**Fix**: Shorten description or move details to body

```text
# Before (78 chars)
feat(SRVKP-123): add comprehensive webhook controller with GitHub support

# After (45 chars)
feat(SRVKP-123): add webhook controller

Add comprehensive GitHub integration with push event support.
```text

### Error: "Title has trailing punctuation (.)"

**Cause**: Subject line ends with period or other punctuation

**Fix**: Remove trailing punctuation

```text
# Before
feat(webhook): add handler.

# After
feat(webhook): add handler
```text

### Error: "Body line exceeds max length (85>72)"

**Cause**: Body line too long

**Fix**: Wrap text at 72 characters

```text
# Before
This commit implements a comprehensive webhook handler for GitHub with validation.

# After
This commit implements a comprehensive webhook handler for GitHub
with validation and error handling.
```text

### Error: "Title does not match regex"

**Cause**: Not following conventional commits format

**Fix**: Use valid type, scope, colon, and description

```text
# Before
Add webhook handler
Feature: add webhook handler
feat add webhook handler

# After
feat(webhook): add handler
```text

### Error: "Body message missing required signature"

**Cause**: Missing Signed-off-by trailer

**Fix**: Add required trailer

```text
feat(webhook): add handler

Add webhook support for GitHub push events.

Signed-off-by: John Developer <john@redhat.com>
```text

## Gitlint Configuration

### Project Configuration

Gitlint configuration is typically in `.gitlint` or `setup.cfg`:

```ini
[general]
ignore=body-is-missing

[title-max-length]
line-length=72

[title-must-not-contain-word]
words=wip,WIP,todo,TODO

[body-max-line-length]
line-length=72

[body-min-length]
min-length=0
```text

### Custom Rules

This project may have custom gitlint rules for:

- Conventional commits enforcement
- Required trailers (Signed-off-by)
- Scope validation
- Jira issue format validation

## CI/CD Integration

### GitHub Actions

Gitlint runs in CI pipeline:

```yaml
- name: Lint commit messages
  run: gitlint --commits "origin/${{ github.base_ref }}..HEAD"
```text

**All commits in PR must pass gitlint before merge**.

### Pre-push Hooks

Configured via `.pre-commit-config.yaml`:

```yaml
- repo: https://github.com/jorisroovers/gitlint
  rev: v0.19.1
  hooks:
    - id: gitlint
      stages: [commit-msg]
```text

## Best Practices

1. **Check before committing**: Run `gitlint` locally before push
2. **Install pre-commit hooks**: Catch issues early with `pre-commit install`
3. **Follow 50/72 rule**: 50 chars for subject (soft), 72 for body (hard)
4. **Use conventional commits**: Always start with valid type
5. **Include required trailers**: Signed-off-by is mandatory
6. **No trailing punctuation**: Subject line ends without period
7. **Blank line separation**: Always separate subject and body
8. **Wrap body text**: Keep all lines under 72 characters

## Troubleshooting

### Gitlint Not Running

**Check**:

1. Pre-commit hooks installed: `pre-commit install --install-hooks`
2. Gitlint in hooks config: Check `.pre-commit-config.yaml`
3. Hook not skipped: Avoid `--no-verify`

### Gitlint Failing in CI but Passing Locally

**Possible causes**:

1. Different gitlint versions
2. Different configuration files
3. CI checks additional commits

**Fix**: Match CI environment locally

### Need to Skip Gitlint Temporarily

**Not recommended**, but for emergency:

```bash
# Skip all hooks
git push --no-verify

# Skip gitlint only
SKIP=gitlint git push
```text

**Document reason in commit or PR**.

## Resources

- [Conventional Commits Specification](https://www.conventionalcommits.org/)
- [Gitlint Documentation](https://jorisroovers.com/gitlint/)
- [Pre-commit Framework](https://pre-commit.com/)

## Summary

Gitlint enforces:

- ✓ Conventional commits format (type, scope, description)
- ✓ Line length limits (72 chars for subject and body)
- ✓ Proper spacing (blank line between subject and body)
- ✓ No trailing punctuation (except `!` for breaking changes)
- ✓ Required trailers (Signed-off-by)
- ✓ Valid commit types (feat, fix, docs, etc.)

Follow these rules to ensure commits pass validation in both local and CI environments.
