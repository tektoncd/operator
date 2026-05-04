# Commit Types Reference

Complete reference for conventional commit types used in the Tekton Operator project.

## Standard Types

### feat - New Features

Use for new features or functionality added to the codebase.

**Examples**:

- `feat(webhook): add GitHub App support`
- `feat(SRVKP-123): implement GitLab integration`
- `feat(controller): add pipeline caching mechanism`
- `feat(api): expose webhook configuration endpoint`

**When to use**:

- Adding new capabilities
- Introducing new components
- Implementing new APIs or endpoints
- Adding new configuration options

### fix - Bug Fixes

Use for bug fixes that resolve incorrect behavior.

**Examples**:

- `fix(controller): resolve pipeline race condition`
- `fix(SRVKP-456): correct webhook payload parsing`
- `fix(matcher): handle edge case in regex matching`
- `fix(api): prevent nil pointer dereference`

**When to use**:

- Fixing crashes or errors
- Resolving incorrect behavior
- Patching security vulnerabilities
- Correcting logic errors

### docs - Documentation

Use for documentation-only changes.

**Examples**:

- `docs(README): update installation steps`
- `docs(api): add webhook endpoint documentation`
- `docs(SRVKP-789): document GitLab setup process`
- `docs(contributing): add code review guidelines`

**When to use**:

- Updating README files
- Adding or improving code comments
- Updating API documentation
- Improving developer guides

### refactor - Code Refactoring

Use for code changes that neither fix bugs nor add features, but improve code structure.

**Examples**:

- `refactor(controller): extract reconciliation logic`
- `refactor(matcher): simplify regex patterns`
- `refactor(webhook): consolidate handler functions`
- `refactor(SRVKP-234): reorganize package structure`

**When to use**:

- Improving code readability
- Simplifying complex logic
- Reorganizing code structure
- Extracting common functionality

### test - Testing

Use for adding or updating tests.

**Examples**:

- `test(webhook): add integration tests`
- `test(controller): improve coverage for edge cases`
- `test(SRVKP-567): add E2E test for GitLab`
- `test(matcher): add regex validation tests`

**When to use**:

- Adding new test cases
- Improving test coverage
- Fixing flaky tests
- Adding integration or E2E tests

### chore - Maintenance Tasks

Use for routine maintenance tasks and tooling.

**Examples**:

- `chore(deps): update go dependencies`
- `chore(vendor): run make vendor`
- `chore(tools): update pre-commit hooks`
- `chore(SRVKP-890): update golangci-lint version`

**When to use**:

- Dependency updates
- Tooling configuration
- Repository maintenance
- Build script updates (minor)

### build - Build System

Use for changes to build system or external dependencies.

**Examples**:

- `build(Makefile): add vendor target`
- `build(docker): update container base image`
- `build(ko): configure image build settings`
- `build(SRVKP-345): add arm64 build support`

**When to use**:

- Makefile changes
- Docker/container build changes
- Dependency management (major)
- Build configuration updates

### ci - CI/CD Changes

Use for changes to CI/CD configuration and scripts.

**Examples**:

- `ci(github): add golangci-lint action`
- `ci(actions): update test workflow`
- `ci(SRVKP-678): add GitLab CI configuration`
- `ci(pre-commit): add new validation hooks`

**When to use**:

- GitHub Actions updates
- GitLab CI configuration
- Pre-commit hook changes
- CI pipeline modifications

### perf - Performance Improvements

Use for changes that improve performance.

**Examples**:

- `perf(cache): optimize lookup speed`
- `perf(controller): reduce reconciliation time`
- `perf(SRVKP-901): implement pipeline caching`
- `perf(matcher): improve regex compilation`

**When to use**:

- Optimization work
- Reducing latency
- Improving throughput
- Memory usage improvements

### style - Code Style

Use for formatting and style changes that don't affect code behavior.

**Examples**:

- `style(format): run fumpt formatter`
- `style(lint): fix golangci-lint warnings`
- `style(markdown): fix markdownlint issues`
- `style(SRVKP-123): apply consistent naming`

**When to use**:

- Running code formatters
- Fixing linter warnings (style-only)
- Applying consistent naming
- Whitespace/indentation fixes

### revert - Revert Previous Commit

Use for reverting previous commits.

**Examples**:

- `revert: undo breaking API change`
- `revert(SRVKP-456): revert webhook refactoring`
- `revert: "feat(api): add new endpoint"`

**When to use**:

- Undoing problematic changes
- Rolling back breaking changes
- Reverting commits that caused issues

**Format**: Include reference to original commit in body

## Breaking Changes

For any commit type, add `!` after type/scope to indicate breaking change:

**Examples**:

- `feat(api)!: change webhook payload format`
- `fix(controller)!: remove deprecated reconciliation mode`
- `refactor(webhook)!: restructure event handlers`

**Body should include**:

```markdown
BREAKING CHANGE: <description of breaking change and migration path>
```

## Type Selection Guide

### Decision Tree

1. **Does it add new functionality?** → `feat`
2. **Does it fix a bug?** → `fix`
3. **Is it documentation only?** → `docs`
4. **Does it change code structure without behavior change?** → `refactor`
5. **Is it test-related?** → `test`
6. **Is it dependency/maintenance?** → `chore`
7. **Is it build system related?** → `build`
8. **Is it CI/CD related?** → `ci`
9. **Does it improve performance?** → `perf`
10. **Is it formatting/style only?** → `style`
11. **Does it revert a previous commit?** → `revert`

### Common Mistakes

**Wrong**: `chore(api): add new endpoint`
**Right**: `feat(api): add new endpoint`
*Reason: Adding endpoint is a feature, not maintenance*

**Wrong**: `feat(tests): add test coverage`
**Right**: `test(controller): add test coverage`
*Reason: Tests have their own type*

**Wrong**: `fix(docs): update README`
**Right**: `docs(README): update installation steps`
*Reason: Documentation has its own type*

**Wrong**: `chore(ci): update GitHub Actions`
**Right**: `ci(github): update test workflow`
*Reason: CI changes have their own type*

## Multiple Changes in One Commit

When a commit includes multiple types of changes, choose the most significant:

**Example**: Adding feature + tests

- **Choose**: `feat(webhook): add GitHub App support`
- **Body mentions**: "Includes integration tests"

**Example**: Bug fix + refactoring

- **Choose**: `fix(controller): resolve race condition`
- **Body mentions**: "Refactored reconciliation logic for clarity"

**Guideline**: If changes are too diverse, consider splitting into multiple commits.
