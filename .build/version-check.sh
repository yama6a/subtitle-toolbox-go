#!/usr/bin/env bash
# Fails a PR whose go.mod module path or imports do not match the major version it releases.
# Reads the PR labels as a JSON list from LABELS. Runs in the repo root and needs the tags.
set -euo pipefail

base=github.com/yama6a/subtitle-toolbox-go

fail() {
  echo "$1" >&2
  exit 1
}

# A missing or double release label is the job of the release label check, so it counts as none here.
label="$(jq -r '[.[] | select(IN("major", "minor", "patch", "skip-release"))]
  | if length == 1 then .[0] else "none" end' <<< "${LABELS:-[]}")"

tags="$(git tag -l 'v*' --sort=-v:refname)"
tag="$(grep -m 1 -E '^v[0-9]+\.[0-9]+\.[0-9]+$' <<< "$tags" || true)"

if [ -z "$tag" ]; then
  case "$label" in
    patch | minor) fail "No v1.0.0 tag yet. Before 1.0.0 a PR carries skip-release, and only the 1.0.0 PR carries major." ;;
    skip-release | major) major=1 ;;
    *) major=0 ;;
  esac
else
  major="${tag#v}"
  major="$((10#${major%%.*}))"
  if [ "$label" = major ]; then
    major=$((major + 1))
  fi
fi

expected="$base"
if [ "$major" -ge 2 ]; then
  expected="$base/v$major"
fi

actual="$(awk '$1 == "module" { gsub(/"/, "", $2); print $2; exit }' go.mod)"
if [ "$actual" != "$expected" ]; then
  if [ "$label" = major ]; then
    fail "A PR labelled major releases v$major.0.0, so go.mod must say \"module $expected\". It says \"$actual\"."
  fi
  fail "go.mod says \"module $actual\", but this PR does not release a new major. Keep \"module $expected\"."
fi

mapfile -t files < <(git ls-files -- '*.go' ':(exclude,glob)**/testdata/**')
if [ "${#files[@]}" -gt 0 ]; then
  violations="$(awk -v base="$base" -v expected="$expected" -v major="$major" '
    FNR == 1 { block = 0 }
    /^import[ \t]*\(/ { block = 1; next }
    block && /^\)/ { block = 0; next }
    block && $1 ~ /^\/\// { next }
    (block || /^import[ \t]/) && match($0, /"[^"]*"/) {
      path = substr($0, RSTART + 1, RLENGTH - 2)
      if (path != base && index(path, base "/") != 1) next
      if (major < 2) {
        bad = substr(path, length(base) + 1) ~ /^\/v[0-9]+(\/|$)/
      } else {
        bad = path != expected && index(path, expected "/") != 1
      }
      if (bad) printf "%s:%d imports \"%s\", which is not under %s.\n", FILENAME, FNR, path, expected
    }' "${files[@]}")"
  if [ -n "$violations" ]; then
    fail "$violations"
  fi
fi

echo "version check: label $label, expected major $major, module $expected"
