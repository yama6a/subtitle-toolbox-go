#!/usr/bin/env bash
# Fails a PR whose release label is weaker than its exported API change.
# Reads the PR labels as a JSON list from LABELS. Runs in the repo root and needs the tags.
set -euo pipefail

# Renovate updates this pin through the custom manager in renovate.json5.
gorelease=golang.org/x/exp/cmd/gorelease@v0.0.0-20261014143641-f0251eb4aa3e

# A missing or double release label is the job of the release label check, so it counts as none here.
label="$(jq -r '[.[] | select(IN("major", "minor", "patch", "skip-release"))]
  | if length == 1 then .[0] else "none" end' <<< "${LABELS:-[]}")"

if [ "$label" = none ]; then
  echo "gorelease: no single release label, nothing to check"
  exit 0
fi

tag="$(git tag -l 'v*' --sort=-v:refname | grep -m 1 -E '^v[0-9]+\.[0-9]+\.[0-9]+$' || true)"
if [ -z "$tag" ]; then
  echo "gorelease: no v* release tag found. Check out with fetch-depth: 0 so the tags are there." >&2
  exit 1
fi

IFS=. read -r major minor patch <<< "${tag#v}"
major=$((10#$major))
minor=$((10#$minor))
patch=$((10#$patch))

# On a major PR go.mod already has the new /vN suffix, but the base is the old module.
module="$(awk '$1 == "module" { gsub(/"/, "", $2); print $2; exit }' go.mod | sed -E 's#/v[0-9]+$##')"

case "$label" in
  patch | skip-release)
    base="$tag"
    proposed="v$major.$minor.$((patch + 1))"
    ;;
  minor)
    base="$tag"
    proposed="v$major.$((minor + 1)).0"
    ;;
  major)
    base="$module@$tag"
    if [ "$major" -ge 2 ]; then
      base="$module/v$major@$tag"
    fi
    proposed="v$((major + 1)).0.0"
    ;;
esac

echo "gorelease: label $label, base $base, proposed $proposed"
rc=0
report="$(go run "$gorelease" -base="$base" -version="$proposed" 2>&1)" || rc=$?
echo "$report"
if [ "$rc" -ne 0 ]; then
  # gorelease also exits 1 for errors that no label fixes, such as a go.mod without the /vN suffix.
  if grep -qE 'There are incompatible changes\.|There are compatible changes, but the minor version is not incremented' <<< "$report"; then
    echo "The API change needs a stronger release label than $label." >&2
  else
    echo "gorelease failed. Read its report above." >&2
  fi
  exit 1
fi
