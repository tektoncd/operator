#!/usr/bin/env bash
set -e -o pipefail

TEKTON_CATALOG_DIR="../tektoncd-catalog"
FETCH_SCRIPT="hack/openshift/fetch-tektoncd-catalog-tasks.sh"

# Check if the tektoncd-catalog repo exists
if [[ ! -d "$TEKTON_CATALOG_DIR" ]]; then
  echo "Error: tektoncd-catalog repo is missing! Clone it first."
  exit 1
fi

# Backup the original fetch script before modifying
cp "$FETCH_SCRIPT" "$FETCH_SCRIPT.bak"

# Function to update versions
update_versions() {
  local type=$1
  local dir_path="$TEKTON_CATALOG_DIR/$type"

  echo "Updating $type versions..."

  for resource_dir in "$dir_path"/*; do
    if [[ ! -d "$resource_dir" ]]; then
      continue
    fi

    resource_name=$(basename "$resource_dir")
    latest_version=$(find "$resource_dir" -maxdepth 1 -type d | grep -v "^$resource_dir$" | sort -V | tail -n 1 | xargs basename)

    if [[ -z "$latest_version" ]]; then
      echo "Skipping $resource_name (no versions found)"
      continue
    fi

    # Update the script with the latest version
    # Using perl for cross-platform compatibility (works on both Linux and macOS)
    perl -i -pe "s|(\\['$resource_name'\\]=)['\"][^'\"]*['\"]|\1'$latest_version'|" "$FETCH_SCRIPT"

    echo "Updated $resource_name to version $latest_version"
  done
}

# Update task versions
update_versions "tasks" "TEKTON_ECOSYSTEM_TASKS"


# Check if there are any changes
if git diff --quiet "$FETCH_SCRIPT"; then
  echo "No version updates found."
  exit 0
else
  echo "Ecosystem Task versions updated successfully!"
fi
