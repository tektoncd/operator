#!/usr/bin/env bash
#
# find-tag-lte.sh - shell function that finds the highest tag in a GitHub repo
# that is semantically less than or equal to a given tag.
#
# Use as a library:
#   source ./find-tag-lte.sh
#   tag="$(find_tag_lte owner/repo v1.2.2)" || echo "none found"
#   tag="$(find_tag_lte owner/repo v1.2.2 --include-prerelease)"
#
# Or run directly:
#   ./find-tag-lte.sh owner/repo v1.2.2 [--include-prerelease]
#
# Prints the matching tag on stdout. Errors go to stderr; returns non-zero
# if nothing matches. Requires: curl, jq, sort with -V support.
# Auth: set GITHUB_TOKEN for private repos or higher rate limits.

find_tag_lte() {
  local repo="${1:-}" input="${2:-}" include_pre=0
  [[ "${3:-}" == "--include-prerelease" ]] && include_pre=1

  local semver_re='^v?[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'

  if [[ -z "$repo" || -z "$input" ]]; then
    echo "usage: find_tag_lte <owner/repo> <vX.Y.Z> [--include-prerelease]" >&2
    return 2
  fi
  [[ "$repo" =~ ^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$ ]] \
    || { echo "error: invalid repo '$repo'" >&2; return 2; }
  [[ "$input" =~ $semver_re ]] \
    || { echo "error: invalid tag '$input' (expected form v1.2.2)" >&2; return 2; }
  command -v curl >/dev/null && command -v jq >/dev/null \
    || { echo "error: curl and jq are required" >&2; return 2; }

  # Drop "v"; map "-" to "~" so 1.2.3-rc.1 sorts below 1.2.3 under sort -V.
  _ftl_norm() { local k="${1#v}"; printf '%s' "${k//-/\~}"; }

  local auth=()
  [[ -n "${GITHUB_TOKEN:-}" ]] && auth=(-H "Authorization: Bearer ${GITHUB_TOKEN}")

  # Fetch all tag names, paginated.
  local names="" page=1 chunk
  while :; do
    chunk="$(curl -fsSL "${auth[@]+"${auth[@]}"}" \
        -H "Accept: application/vnd.github+json" \
        "https://api.github.com/repos/${repo}/tags?per_page=100&page=${page}" \
        | jq -r '.[].name')" \
      || { echo "error: failed to list tags for $repo (bad repo, rate limit, or missing GITHUB_TOKEN?)" >&2
           unset -f _ftl_norm; return 1; }
    [[ -z "$chunk" ]] && break
    names+="${chunk}"$'\n'
    page=$((page + 1))
  done

  local input_key best="" tag key
  input_key="$(_ftl_norm "$input")"

  # Keep semver-shaped tags (skip pre-releases unless asked), compare to input.
  local candidates=""
  while IFS= read -r tag; do
    [[ "$tag" =~ $semver_re ]] || continue
    [[ $include_pre -eq 0 && "$tag" == *-* ]] && continue
    key="$(_ftl_norm "$tag")"
    # key <= input_key  iff key sorts first (or equal) when version-sorted
    if [[ "$(printf '%s\n%s\n' "$key" "$input_key" | sort -V | head -n1)" == "$key" ]]; then
      candidates+="${key}"$'\t'"${tag}"$'\n'
    fi
  done <<<"$names"

  unset -f _ftl_norm

  if [[ -z "$candidates" ]]; then
    echo "error: no tag in $repo is <= $input" >&2
    return 1
  fi

  best="$(printf '%s' "$candidates" | sort -t$'\t' -k1,1V | tail -n1 | cut -f2)"
  printf '%s\n' "$best"
}

# Run directly (not when sourced).
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  find_tag_lte "$@"
fi

