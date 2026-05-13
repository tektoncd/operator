---
name: release-notes
description: This skill should be used when the user asks to "create release note", "generate release notes", "release notes", "release changelog", "update GitHub release", or wants to generate categorized release notes between tags. Gathers PR/commit data via gh CLI, categorizes changes, and outputs formatted release notes. The user can optionally specify a release version (e.g. "create release note v0.13.0") to bypass auto-detection.
version: 0.1.0
---

# Release Notes Generation

Generate categorized release notes for Tekton Operator releases by gathering PR/commit data between two git tags and producing formatted markdown output.

## Purpose

Claude-native skill that:

- Auto-detects current and previous tags
- Gathers PR and commit data via `gh` CLI
- Uses Claude itself for intelligent categorization (no external AI API needed)
- Outputs release notes matching the project's established format
- Optionally updates the GitHub release

## Workflow

### Step 1: Pull latest tags

```bash
git pull origin --tags
```

Ensure all tags are available locally before detection.

### Step 2: Detect repository info

```bash
gh repo view --json owner,name
```

Extract `owner` and `repo` name for API calls.

### Step 3: Detect tags

**If the user specified a version** (e.g. "create release note v0.31.0"), use that directly as the current tag — skip auto-detection. Validate that the tag exists locally:

```bash
git tag --list 'v0.31.0'
```

If the tag doesn't exist locally, error and ask the user to verify.

**Otherwise, auto-detect current tag:**

```bash
git tag --points-at HEAD
```

Filter for `v*` prefixed tags. If no tag points at HEAD, list recent tags and ask the user which tag to use:

```bash
git tag --list 'v*' --sort=-version:refname | head -10
```

**Previous tag:**

```bash
git tag --list 'v*' --sort=-version:refname
```

Find the entry immediately after the current tag in the version-sorted list.

**CRITICAL**: Confirm both tags with the user before proceeding. Display:

```text
Current tag:  v0.31.0
Previous tag: v0.30.0

Proceed with these tags? (y/n)
```

### Step 4: Verify prerequisites

Check `gh` authentication:

```bash
gh auth status
```

If not authenticated, instruct the user to run `gh auth login`.

**Validate that both tags exist on GitHub:**

```bash
gh api repos/{owner}/{repo}/git/ref/tags/{current_tag}
gh api repos/{owner}/{repo}/git/ref/tags/{previous_tag}
```

If either tag does not exist on GitHub, **stop and report the error** — do not proceed.

**Check GitHub release status for the current tag:**

```bash
gh release view {current_tag} --json isDraft,isPrerelease,tagName
```

- If **no release exists** for the tag: warn the user, but allow proceeding (notes can be generated for preview, but cannot be uploaded).
- If the release is a **draft** (`isDraft: true`): proceed normally — this is the expected state for generating release notes.
- If the release is **published** (not draft, not prerelease): **ask for explicit confirmation** before proceeding, since updating will override an already-published release:

```text
⚠️  The release for {current_tag} is already published (not a draft).
Updating it will override the existing release notes.
Are you sure you want to proceed? (y/n)
```

Only continue if the user confirms.

### Step 5: Gather PR/commit data

Use `gh api` to collect data between the two tags:

**Compare commits:**

```bash
gh api repos/{owner}/{repo}/compare/{previous_tag}...{current_tag} --jq '.commits[].sha'
```

**For each commit, find associated PRs:**

```bash
gh api repos/{owner}/{repo}/commits/{sha}/pulls
```

**Deduplicate by PR number.** For each PR/commit, extract:

- PR number, title, body, author, URL, labels
- JIRA tickets using regex pattern: `SRVKP-\d+` (search in PR title, PR body, and commit message)

For commits not associated with any PR, include them as standalone commit entries with their SHA, message, author, and URL.

**IMPORTANT**: This step involves many API calls. Process commits in reasonable batches and report progress to the user.

### Step 6: Categorize changes

Using the gathered PR/commit data, categorize each entry into exactly these sections (skip empty ones):

- `## ✨ Major changes and Features`
- `## 🐛 Bug Fixes`
- `## 📚 Documentation Updates`
- `## ⚙️ Chores`

Use the entry format specified in `references/release-notes-format.md`.

**Categorization guidelines:**

- New capabilities, enhancements, significant behavior changes → Features
- Bug fixes, error corrections, regression fixes → Bug Fixes
- Documentation-only changes (docs/, README, comments) → Documentation Updates
- Dependencies, CI/CD, refactoring, formatting, test-only changes → Chores
- Within each section, entries with JIRA tickets go FIRST, before entries without JIRA tickets

**Internal vs user-facing detection:**

Changes that match ANY of these patterns are internal and belong in Chores, NOT Features — regardless of `feat:` prefix:

- CI/CD pipeline changes (`.tekton/`, `.github/workflows/`, Makefile targets)
- Release infrastructure (release scripts, release pipeline tasks, goreleaser config)
- Test infrastructure (test helpers, e2e framework, test configuration)
- Build system changes (Dockerfile, ko config, build scripts)
- Developer tooling (linter config, pre-commit hooks, code generation)
- Internal refactoring that doesn't change user-visible behavior

Only classify as Features when the change is **visible to end users**: new CLI flags, new API fields, new provider capabilities, new webhook behaviors, new configuration options, new user-facing commands.

### Step 7: Assemble complete release notes

Combine all sections in this order:

1. **Header** (see `references/release-notes-format.md` for template)
2. **Categorized sections** from Step 6
3. **Installation section** (see `references/release-notes-format.md` for template)
4. **GitHub auto-generated changelog**:

```bash
gh api repos/{owner}/{repo}/releases/generate-notes -f tag_name="{current_tag}" -f previous_tag_name="{previous_tag}"
```

Extract the `body` field from the response.

### Step 8: Output, save, and optional GitHub release update

1. **Always write** the complete release notes to `/tmp/release-notes-{current_tag}.md` and tell the user the file path.
2. **Display** the complete release notes to the user.
3. **Ask** if they want to update the GitHub release:

```text
Release notes saved to /tmp/release-notes-{current_tag}.md

Would you like to update the GitHub release for {current_tag} with these notes? (y/n)
```

1. If yes, update via:

```bash
gh release edit {current_tag} --notes-file /tmp/release-notes-{current_tag}.md
```

If the release has a TODO placeholder (matching pattern `TODO: XXXXX.*?see older releases for some example`), replace only that placeholder in the existing release body rather than overwriting the entire body.

### Step 9: Slack announcement (optional)

After Step 8, ask the user if they want a Slack announcement message.

If yes:

1. Generate a friendly summary with a few emojis (not excessive), listing the top 5-7 most important features and/or bug fixes. Max 7 items total.
2. Save to `/tmp/release-notes-slack-{current_tag}.txt` and tell the user the file path so they can copy-paste.
3. Append the GitHub release URL at the end of the message.

## Error Handling

| Scenario | Action |
| --- | --- |
| No tag at HEAD | List recent tags, ask user to pick one |
| User-specified tag doesn't exist locally | Error and ask user to verify the tag name |
| Tag doesn't exist on GitHub | Stop and report error — do not proceed |
| `gh` not authenticated | Instruct user to run `gh auth login` |
| No previous tag found | Ask user to provide one manually |
| No GitHub release for tag | Warn but allow generating notes for preview |
| Release is a draft | Proceed normally (expected state) |
| Release is already published | Ask for explicit confirmation before overriding |
| API rate limiting | Report the error, suggest waiting or using a different token |

## User Confirmation Requirements

**CRITICAL**: Always confirm tags before gathering data. Always confirm before updating a GitHub release. Never update a release without explicit user approval.
