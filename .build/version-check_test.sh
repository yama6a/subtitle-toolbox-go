#!/usr/bin/env bash
# Runs .build/version-check.sh in a new git repo per case and checks its exit code and message.
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/version-check.sh"
base=github.com/yama6a/subtitle-toolbox-go
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# The test repos must not pick up the signing key or hooks of the user running the test.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1

v1_src="package pkg

import (
	\"fmt\"

	\"$base/model\"
)"

v2_src="package pkg

import \"$base/v2/model\""

# Every repo also holds this file. It must never fail rule 9, because it sits under testdata.
testdata_src="package old

import \"$base/v9/model\""

cases=0
failures=0

# check <name> <LABELS> <tags, space separated> <module path> <pkg/pkg.go source> <want exit> <want line>
check() {
  local name=$1 labels=$2 module=$4 src=$5 want_rc=$6 want=$7
  local tags tag repo out line rc=0
  read -ra tags <<< "$3"
  cases=$((cases + 1))
  repo="$work/$cases"
  mkdir -p "$repo/pkg/testdata/old"
  printf 'module %s\n\ngo 1.27.2\n' "$module" > "$repo/go.mod"
  printf '%s\n' "$src" > "$repo/pkg/pkg.go"
  printf '%s\n' "$testdata_src" > "$repo/pkg/testdata/old/old.go"
  git -C "$repo" init -q -b main
  git -C "$repo" add -A
  git -C "$repo" -c user.name=test -c user.email=test@example.invalid commit -q -m test
  for tag in "${tags[@]}"; do
    git -C "$repo" tag "$tag"
  done

  out="$(cd "$repo" && LABELS="$labels" "$script" 2>&1)" || rc=$?
  if [ "$rc" -eq "$want_rc" ] && grep -qxF -- "$want" <<< "$out"; then
    echo "ok   $name"
    return
  fi
  failures=$((failures + 1))
  echo "FAIL $name"
  echo "  exit $rc, want $want_rc"
  echo "  want line: $want"
  while IFS= read -r line; do
    echo "  got: $line"
  done <<< "$out"
}

pass() {
  echo "version check: label $1, expected major $2, module $3"
}

check "rule 1: no tag, patch" '["patch"]' "" "$base" "$v1_src" 1 \
  "No v1.0.0 tag yet. Before 1.0.0 a PR carries skip-release, and only the 1.0.0 PR carries major."
check "rule 1: no tag, minor" '["minor","infra"]' "" "$base" "$v1_src" 1 \
  "No v1.0.0 tag yet. Before 1.0.0 a PR carries skip-release, and only the 1.0.0 PR carries major."
check "rule 2: no tag, skip-release" '["skip-release"]' "" "$base" "$v1_src" 0 \
  "$(pass skip-release 1 "$base")"
check "rule 2: no tag, major" '["major"]' "" "$base" "$v1_src" 0 \
  "$(pass major 1 "$base")"
check "rule 3: v1 tag, major" '["major"]' "v1.4.2" "$base/v2" "$v2_src" 0 \
  "$(pass major 2 "$base/v2")"
check "rule 4: v1 tag, patch" '["patch"]' "v1.4.2" "$base" "$v1_src" 0 \
  "$(pass patch 1 "$base")"
check "rule 4: v2 tag, no label" '[]' "v2.3.0" "$base/v2" "$v2_src" 0 \
  "$(pass none 2 "$base/v2")"
check "rule 4: two release labels count as none" '["patch","major"]' "v1.4.2" "$base" "$v1_src" 0 \
  "$(pass none 1 "$base")"
check "rule 4: newest tag by version, pre-releases and other tags ignored" '["minor"]' \
  "v9.1.0 v10.0.0 v11.0.0-rc.1 release-12" "$base/v10" "package pkg" 0 \
  "$(pass minor 10 "$base/v10")"
check "rule 5: no tag, no label" '[]' "" "$base" "$v1_src" 0 \
  "$(pass none 0 "$base")"
check "rule 6: v2 tag, major" '["major"]' "v2.1.0" "$base/v3" "package pkg" 0 \
  "$(pass major 3 "$base/v3")"
check "rule 7: major without the new module path" '["major"]' "v1.4.2" "$base" "$v1_src" 1 \
  "A PR labelled major releases v2.0.0, so go.mod must say \"module $base/v2\". It says \"$base\"."
check "rule 8: new module path without major" '["patch"]' "v1.4.2" "$base/v2" "$v2_src" 1 \
  "go.mod says \"module $base/v2\", but this PR does not release a new major. Keep \"module $base\"."
check "rule 9: v1 imports a v2 package in a block" '["patch"]' "v1.4.2" "$base" \
  "$(printf 'package pkg\n\nimport (\n\t"fmt"\n\n\tm "%s/v2/model"\n)' "$base")" 1 \
  "pkg/pkg.go:6 imports \"$base/v2/model\", which is not under $base."
check "rule 9: v1 imports the v2 root" '["skip-release"]' "" "$base" \
  "$(printf 'package pkg\n\nimport "%s/v2"' "$base")" 1 \
  "pkg/pkg.go:3 imports \"$base/v2\", which is not under $base."
check "rule 9: v2 imports a v1 package" '["minor"]' "v2.0.0" "$base/v2" \
  "$(printf 'package pkg\n\nimport "%s/model"' "$base")" 1 \
  "pkg/pkg.go:3 imports \"$base/model\", which is not under $base/v2."

echo "$cases cases, $failures failed"
[ "$failures" -eq 0 ]
