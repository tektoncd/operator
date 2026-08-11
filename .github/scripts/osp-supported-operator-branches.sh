#!/usr/bin/env bash
# Print a JSON array of OSP-supported operator release branches
# (branches.operator.upstream from openshift-pipelines/hack release configs).
# Source: https://github.com/openshift-pipelines/hack/tree/main/config/downstream/releases
#
# Fetches only the release YAML files via the GitHub Contents API (not the
# full repo tarball). Set GITHUB_TOKEN to avoid anonymous API rate limits.
set -euo pipefail

API_URL="https://api.github.com/repos/openshift-pipelines/hack/contents/config/downstream/releases"

AUTH_ARGS=()
if [[ -n "${GITHUB_TOKEN:-}" ]]; then
  AUTH_ARGS=(-H "Authorization: Bearer ${GITHUB_TOKEN}")
fi

BRANCHES=$(
  curl -fsSL \
    -H "Accept: application/vnd.github+json" \
    "${AUTH_ARGS[@]}" \
    "${API_URL}" \
    | jq -r '.[].download_url // empty' \
    | while read -r url; do
        [[ -n "${url}" ]] || continue
        curl -fsSL "${url}"
      done \
    | awk '
      /^[[:space:]]*operator:[[:space:]]*$/ { in_op=1; next }
      in_op && /^[[:space:]]*upstream:[[:space:]]*/ {
        sub(/^[[:space:]]*upstream:[[:space:]]*/, "")
        gsub(/[[:space:]].*/, "")
        if ($0 ~ /^release-/) print
        in_op=0
      }
      in_op && NF && !/^[[:space:]]*#/ && !/^[[:space:]]*upstream:/ { in_op=0 }
    ' \
    | sort -u
)

[[ -n "${BRANCHES}" ]] || {
  echo "error: no OSP operator release branches found" >&2
  exit 1
}

printf '%s\n' "${BRANCHES}" | jq -R . | jq -sc .
