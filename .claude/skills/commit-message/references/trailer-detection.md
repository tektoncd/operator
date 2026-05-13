# Author Detection and Trailer Generation

Complete guide for detecting author information and generating required commit trailers.

## Overview

Every commit must include two trailers:

1. **Signed-off-by**: Author's name and email
2. **Assisted-by**: AI model information (when AI assists)

This document describes the complete detection logic and trailer generation process.

## Signed-off-by Trailer

### Purpose

The `Signed-off-by` trailer indicates who created the commit and certifies that the contributor has the right to submit the code.

**Format**: `Signed-off-by: Full Name <email@example.com>`

### Detection Priority Order

Author information is detected using this priority:

#### Priority 1: Environment Variables (Highest)

Check environment variables first (common in dev containers and CI):

```bash
# Check if both variables are set
if [ -n "$GIT_AUTHOR_NAME" ] && [ -n "$GIT_AUTHOR_EMAIL" ]; then
    author_name="$GIT_AUTHOR_NAME"
    author_email="$GIT_AUTHOR_EMAIL"
fi
```bash

**Environment variables**:

- `$GIT_AUTHOR_NAME` - Author's full name
- `$GIT_AUTHOR_EMAIL` - Author's email address

**Example**:

```bash
export GIT_AUTHOR_NAME="Jane Developer"
export GIT_AUTHOR_EMAIL="jane.developer@redhat.com"
```bash

**Result**:

```bash
Signed-off-by: Jane Developer <jane.developer@redhat.com>
```bash

**When used**:

- Development containers (devcontainers)
- CI/CD pipelines
- Containerized environments
- Explicitly set environments

#### Priority 2: Git Configuration (Fallback)

If environment variables not set, check git configuration:

```bash
# Get author name from git config
author_name=$(git config user.name)
author_email=$(git config user.email)

if [ -n "$author_name" ] && [ -n "$author_email" ]; then
    # Use git config values
    echo "Using git config: $author_name <$author_email>"
fi
```bash

**Git config sources** (in priority order):

1. **Repository config** (`.git/config`): `git config user.name`
2. **Global config** (`~/.gitconfig`): `git config --global user.name`
3. **System config** (`/etc/gitconfig`): `git config --system user.name`

**Example**:

```bash
# Set globally
git config --global user.name "John Developer"
git config --global user.email "john.developer@redhat.com"

# Or per-repository
git config user.name "John Developer"
git config user.email "john.developer@redhat.com"
```bash

**Result**:

```bash
Signed-off-by: John Developer <john.developer@redhat.com>
```bash

**When used**:

- Standard git installations
- User's local machine
- Normal development workflow

#### Priority 3: Ask User (Last Resort)

If neither environment variables nor git config available:

**Prompt**:

```bash
Git author information not configured. Please provide:

Full Name: _
Email: _
```bash

**Validation**:

- Name: Non-empty, contains at least first and last name
- Email: Valid email format (`user@domain.com`)

**Store for session**:
After user provides information, optionally ask:

```bash
Would you like to save this information?
1. For this repository only (git config)
2. Globally for all repositories (git config --global)
3. Just for this commit (don't save)
```bash

**Example interaction**:

```bash
Git author information not configured. Please provide:

Full Name: Sarah Developer
Email: sarah@example.com

Would you like to save this information?
1. For this repository only (git config)
2. Globally for all repositories (git config --global)
3. Just for this commit (don't save)

Choice: 2

Saving globally...
git config --global user.name "Sarah Developer"
git config --global user.email "sarah@example.com"
Done!
```bash

### Complete Detection Logic

```bash
detect_author() {
    local author_name=""
    local author_email=""

    # Priority 1: Environment variables
    if [ -n "$GIT_AUTHOR_NAME" ] && [ -n "$GIT_AUTHOR_EMAIL" ]; then
        author_name="$GIT_AUTHOR_NAME"
        author_email="$GIT_AUTHOR_EMAIL"
        echo "Using environment variables" >&2

    # Priority 2: Git config
    elif git config user.name >/dev/null && git config user.email >/dev/null; then
        author_name=$(git config user.name)
        author_email=$(git config user.email)
        echo "Using git config" >&2

    # Priority 3: Ask user
    else
        echo "Git author information not configured." >&2
        read -p "Full Name: " author_name
        read -p "Email: " author_email

        # Validate
        if [ -z "$author_name" ] || [ -z "$author_email" ]; then
            echo "Error: Name and email are required" >&2
            return 1
        fi

        # Offer to save
        echo "Would you like to save this information?" >&2
        echo "1. For this repository only (git config)" >&2
        echo "2. Globally for all repositories (git config --global)" >&2
        echo "3. Just for this commit (don't save)" >&2
        read -p "Choice: " choice

        case $choice in
            1)
                git config user.name "$author_name"
                git config user.email "$author_email"
                echo "Saved to repository config" >&2
                ;;
            2)
                git config --global user.name "$author_name"
                git config --global user.email "$author_email"
                echo "Saved to global config" >&2
                ;;
            3)
                echo "Using for this commit only" >&2
                ;;
        esac
    fi

    # Generate Signed-off-by trailer
    echo "Signed-off-by: $author_name <$author_email>"
}
```bash

### Email Validation

Ensure email is in valid format:

**Valid formats**:

- `user@example.com`
- `first.last@company.com`
- `user+tag@example.com`
- `user123@subdomain.example.com`

**Invalid formats**:

- `user@` (missing domain)
- `@example.com` (missing user)
- `user@.com` (missing domain name)
- `user example.com` (missing @)

**Validation regex**: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`

### Common Scenarios

#### Scenario 1: DevContainer with Environment Variables

```bash
# In .devcontainer/devcontainer.json
{
  "containerEnv": {
    "GIT_AUTHOR_NAME": "Developer Name",
    "GIT_AUTHOR_EMAIL": "developer@redhat.com"
  }
}
```bash

**Detection**: Uses environment variables (Priority 1)
**Result**: `Signed-off-by: Developer Name <developer@redhat.com>`

#### Scenario 2: Local Development with Git Config

```bash
# User's ~/.gitconfig
[user]
    name = John Developer
    email = john@example.com
```bash

**Detection**: Uses git config (Priority 2)
**Result**: `Signed-off-by: John Developer <john@example.com>`

#### Scenario 3: Fresh Git Installation

No environment variables, no git config set.

**Detection**: Asks user (Priority 3)
**Interaction**:

```bash
Git author information not configured. Please provide:

Full Name: Sarah Developer
Email: sarah@example.com

Would you like to save this information?
1. For this repository only (git config)
2. Globally for all repositories (git config --global)
3. Just for this commit (don't save)

Choice: 2
```bash

**Result**: `Signed-off-by: Sarah Developer <sarah@example.com>`

#### Scenario 4: Multiple Git Identities

User has different identities for different projects:

```bash
# Global config (personal)
git config --global user.name "Jane Doe"
git config --global user.email "jane@personal.com"

# Repository config (work)
cd /work/project
git config user.name "Jane Developer"
git config user.email "jane.developer@redhat.com"
```bash

**Detection**: Uses repository config (overrides global)
**Result**: `Signed-off-by: Jane Developer <jane.developer@redhat.com>`

## Assisted-by Trailer

### Assisted-by Purpose

The `Assisted-by` trailer credits AI assistance in creating the commit, following open source contribution practices.

**Format**: `Assisted-by: Model Name (via Tool Name)`

### Model Name Detection

Determine the current AI model being used:

**Available models**:

- Claude Sonnet 4.5
- Claude Opus 4.5
- Claude Haiku
- (Other Claude models)

**Detection method**: Check model identifier or configuration

**Examples**:

```bash
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
Assisted-by: Claude Opus 4.5 (via Claude Code)
Assisted-by: Claude Haiku (via Claude Code)
```bash

### Tool Name

Always use: `Claude Code` (for this project)

Other projects might use:

- `Cursor`
- `Continue`
- `Codex CLI`
- Other AI coding tools

### Format Rules

1. **Model name first**: Full model name (e.g., `Claude Sonnet 4.5`)
2. **Tool in parentheses**: `(via Tool Name)`
3. **Consistent casing**: Proper names capitalized
4. **No abbreviations**: Use full names

**Correct**:

```bash
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
```bash

**Incorrect**:

```bash
Assisted-by: claude-sonnet (via claude code)      # Wrong casing
Assisted-by: Sonnet (via CC)                      # Abbreviations
Assisted-by: (via Claude Code) Claude Sonnet 4.5  # Wrong order
Assisted-by: Claude Sonnet 4.5                    # Missing tool
```bash

### When to Include

**Always include** when:

- AI generated commit message
- AI assisted with commit message
- AI helped analyze changes
- AI suggested improvements

**Optional** (user preference) when:

- AI was consulted but not directly used
- Commit message manually written after AI consultation

**Standard practice**: Always include for transparency and credit.

## Complete Trailer Generation

### Both Trailers Together

Always include both trailers in this order:

1. Signed-off-by (required)
2. Assisted-by (when AI assists)

**Format**:

```bash
Signed-off-by: Full Name <email@example.com>
Assisted-by: Model Name (via Tool Name)
```bash

### Complete Example

```bash
feat(SRVKP-456): ensure webhook logs output to stdout

Configure webhook controller to direct all logs to stdout for
container compatibility. This resolves logging issues in Kubernetes
environments where logs are collected from stdout.

Signed-off-by: Jane Developer <jane.developer@redhat.com>
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
```bash

### Spacing Rules

- **Blank line before trailers**: Separate body from trailers
- **No blank lines between trailers**: Trailers are consecutive
- **No trailing blank lines**: End commit message after last trailer

**Correct**:

```bash
feat(webhook): add handler

Implements webhook support.

Signed-off-by: Jane Developer <jane@example.com>
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
```bash

**Incorrect**:

```bash
feat(webhook): add handler

Implements webhook support.
Signed-off-by: Jane Developer <jane@example.com>

Assisted-by: Claude Sonnet 4.5 (via Claude Code)

```bash

(Missing blank line before trailers, extra blank lines between/after)

## Trailer Generation Function

Complete implementation:

```bash
generate_trailers() {
    local author_name=""
    local author_email=""
    local model_name="Claude Sonnet 4.5"  # Detect actual model
    local tool_name="Claude Code"

    # Detect author (Priority 1: env vars)
    if [ -n "$GIT_AUTHOR_NAME" ] && [ -n "$GIT_AUTHOR_EMAIL" ]; then
        author_name="$GIT_AUTHOR_NAME"
        author_email="$GIT_AUTHOR_EMAIL"

    # Priority 2: git config
    elif git config user.name >/dev/null && git config user.email >/dev/null; then
        author_name=$(git config user.name)
        author_email=$(git config user.email)

    # Priority 3: ask user
    else
        read -p "Full Name: " author_name
        read -p "Email: " author_email
    fi

    # Generate trailers
    echo ""  # Blank line before trailers
    echo "Signed-off-by: $author_name <$author_email>"
    echo "Assisted-by: $model_name (via $tool_name)"
}
```bash

## Troubleshooting

### Issue: "user.name" not set

**Error**: `fatal: unable to auto-detect email address`

**Fix**:

```bash
git config --global user.name "Your Name"
git config --global user.email "your.email@example.com"
```bash

### Issue: Wrong author in devcontainer

**Cause**: Environment variables override git config

**Check**:

```bash
echo "$GIT_AUTHOR_NAME <$GIT_AUTHOR_EMAIL>"
```bash

**Fix**: Set correct environment variables in `.devcontainer/devcontainer.json`:

```json
{
  "containerEnv": {
    "GIT_AUTHOR_NAME": "Correct Name",
    "GIT_AUTHOR_EMAIL": "correct@email.com"
  }
}
```bash

### Issue: Trailers in wrong order

**Wrong**:

```bash
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
Signed-off-by: Jane Developer <jane@example.com>
```bash

**Correct**:

```bash
Signed-off-by: Jane Developer <jane@example.com>
Assisted-by: Claude Sonnet 4.5 (via Claude Code)
```bash

**Fix**: Always put Signed-off-by first, Assisted-by second

## Best Practices

1. **Set git config globally**: Configure once for all projects
2. **Use environment variables in containers**: Consistent in devcontainers
3. **Validate email format**: Ensure valid email before using
4. **Always include both trailers**: Transparency and compliance
5. **Consistent model naming**: Use full model names
6. **Proper spacing**: Blank line before trailers, none between
7. **Check detection priority**: Understand which source is used

## Summary

Trailer detection priority:

1. ✓ Environment variables (`$GIT_AUTHOR_NAME`, `$GIT_AUTHOR_EMAIL`)
2. ✓ Git configuration (`git config user.name`, `git config user.email`)
3. ✓ Ask user (last resort)

Required trailers (in order):

1. ✓ `Signed-off-by: Name <email>`
2. ✓ `Assisted-by: Model Name (via Tool Name)`

Following this process ensures consistent, compliant commit trailers across all environments.
