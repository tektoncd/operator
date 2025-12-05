#!/usr/bin/env bash

# This script generates detailed commit messages and PR descriptions
# for component version bumps. It parses the output from the bump command
# and creates a structured message with links to releases.

set -euo pipefail

BUMP_OUTPUT_FILE="${1:-}"
COMMIT_MSG_FILE="${2:-commit-message.txt}"
PR_BODY_FILE="${3:-pr-body.txt}"

if [[ -z "${BUMP_OUTPUT_FILE}" ]]; then
    echo "Usage: $0 <bump-output-file> [commit-message-file] [pr-body-file]"
    exit 1
fi

# Parse the bump output to extract component updates
declare -a UPDATES=()
IN_UPDATES=false

while IFS= read -r line; do
    if [[ "${line}" == "BUMP_UPDATES_START" ]]; then
        IN_UPDATES=true
        continue
    elif [[ "${line}" == "BUMP_UPDATES_END" ]]; then
        IN_UPDATES=false
        continue
    fi

    if [[ "${IN_UPDATES}" == "true" ]]; then
        UPDATES+=("${line}")
    fi
done < "${BUMP_OUTPUT_FILE}"

# Check if there are any updates
if [[ ${#UPDATES[@]} -eq 0 ]]; then
    echo "No component updates found"
    exit 0
fi

# Generate commit message
{
    if [[ ${#UPDATES[@]} -eq 1 ]]; then
        # Single component update
        IFS='|' read -r name old_version new_version github <<< "${UPDATES[0]}"
        echo "chore: bump ${name} from ${old_version} to ${new_version}"
    else
        # Multiple component updates
        echo "chore: bump component versions"
    fi
    echo ""

    # Add detailed list of updates
    for update in "${UPDATES[@]}"; do
        IFS='|' read -r name old_version new_version github <<< "${update}"
        echo "- ${name}: ${old_version} → ${new_version}"
    done

    echo ""
    echo "Signed-off-by: tekton-bot <tekton-bot@users.noreply.github.com>"
} > "${COMMIT_MSG_FILE}"

# Generate PR body
{
    echo "## Component Version Updates"
    echo ""
    echo "This PR updates the following component versions:"
    echo ""

    for update in "${UPDATES[@]}"; do
        IFS='|' read -r name old_version new_version github <<< "${update}"

        # Create release comparison URL
        release_url="https://github.com/${github}/compare/${old_version}...${new_version}"

        echo "### ${name}"
        echo ""
        echo "- **Previous version:** \`${old_version}\`"
        echo "- **New version:** \`${new_version}\`"
        echo "- **Repository:** [${github}](https://github.com/${github})"
        echo "- **Release notes:** [${old_version}...${new_version}](${release_url})"
        echo ""
    done

    echo "---"
    echo ""
    echo "_This PR was automatically created by the bump-payload workflow._"
} > "${PR_BODY_FILE}"

echo "Generated commit message in ${COMMIT_MSG_FILE}"
echo "Generated PR body in ${PR_BODY_FILE}"
