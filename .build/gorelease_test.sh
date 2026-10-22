#!/usr/bin/env bash
# Runs .build/gorelease.sh in a new module and git repo per case and checks its exit code and message.
set -euo pipefail

script="$(cd "$(dirname "$0")" && pwd)/gorelease.sh"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

# The test repos must not pick up the signing key or hooks of the user running the test.
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1
# The base version of each case comes from a file proxy. gorelease itself still comes from the real proxy.
proxy_default="$(go env GOPROXY)"
# A temporary module cache keeps a base zip of an earlier run from replacing the one of this run.
export GOMODCACHE="$work/modcache" GOFLAGS="${GOFLAGS:-} -modcacherw"
export GOPROXY="file://$work/proxy,$proxy_default" GONOSUMDB=example.com

api='package pkg

func Keep() {}

func Change(a int) {}'

api_added="$api

func Added() {}"

api_removed='package pkg

func Keep() {}'

api_changed='package pkg

func Keep() {}

func Change(a, b int) {}'

api_hidden="$api

func hidden() {}"

api_broken="$api

func Broken() { undefined() }"

internal='package util

func Helper() {}'

internal_added="$internal

func Extra() {}"

cases=0
failures=0

# publish <repo> <module> <tag> serves the files of the tag from the file proxy.
publish() {
  local repo=$1 module=$2 tag=$3 dir
  dir="$work/proxy/$module/@v"
  mkdir -p "$dir"
  git -C "$repo" archive --format=zip --prefix="$module@$tag/" -o "$dir/$tag.zip" "$tag"
  git -C "$repo" show "$tag:go.mod" > "$dir/$tag.mod"
  printf '{"Version":"%s","Time":"2026-01-01T00:00:00Z"}\n' "$tag" > "$dir/$tag.info"
  printf '%s\n' "$tag" > "$dir/list"
}

commit() {
  git -C "$1" add -A
  git -C "$1" -c user.name=test -c user.email=test@example.invalid commit -q -m "$2"
}

# check <name> <LABELS> <tag, empty for none> <module suffix at the tag> <module suffix in the PR> <pkg.go in the PR> <internal/util/util.go in the PR> <want exit> <want line>
# The module at the tag has the API in $api. %M in the want line stands for the module path without suffix.
check() {
  local name=$1 labels=$2 tag=$3 old_suffix=$4 new_suffix=$5 src=$6 internal_src=$7 want_rc=$8 want=$9
  local repo module out line rc=0
  cases=$((cases + 1))
  repo="$work/$cases"
  module="example.com/case$cases"
  want="${want//%M/$module}"

  mkdir -p "$repo/internal/util"
  printf 'module %s\n\ngo 1.21\n' "$module$old_suffix" > "$repo/go.mod"
  printf '%s\n' "$api" > "$repo/pkg.go"
  printf '%s\n' "$internal" > "$repo/internal/util/util.go"
  git -C "$repo" init -q -b main
  commit "$repo" old
  if [ -n "$tag" ]; then
    git -C "$repo" tag "$tag"
    publish "$repo" "$module$old_suffix" "$tag"
  fi

  printf 'module %s\n\ngo 1.21\n' "$module$new_suffix" > "$repo/go.mod"
  printf '%s\n' "$src" > "$repo/pkg.go"
  printf '%s\n' "$internal_src" > "$repo/internal/util/util.go"
  commit "$repo" change

  out="$(cd "$repo" && LABELS="$labels" "$script" 2>&1)" || rc=$?
  if [ "$rc" -eq "$want_rc" ] && grep -qxF -- "$want" <<< "$out" \
    && { [[ "$want" == *"stronger release label"* ]] || ! grep -qF "stronger release label" <<< "$out"; }; then
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

weak() {
  echo "The API change needs a stronger release label than $1."
}

ran() {
  echo "gorelease: label $1, base $2, proposed $3"
}

failed="gorelease failed. Read its report above."
skipped="gorelease: no single release label, nothing to check"

check "patch with a new exported identifier fails" '["patch"]' v1.0.0 "" "" \
  "$api_added" "$internal" 1 "$(weak patch)"
check "skip-release with a new exported identifier fails" '["skip-release","infra"]' v1.0.0 "" "" \
  "$api_added" "$internal" 1 "$(weak skip-release)"
check "minor with a new exported identifier passes" '["minor"]' v1.0.0 "" "" \
  "$api_added" "$internal" 0 "$(ran minor v1.0.0 v1.1.0)"
check "minor with a removed exported identifier fails" '["minor"]' v1.0.0 "" "" \
  "$api_removed" "$internal" 1 "$(weak minor)"
check "minor with a changed exported identifier fails" '["minor"]' v1.0.0 "" "" \
  "$api_changed" "$internal" 1 "$(weak minor)"
check "major with the new module path passes" '["major"]' v1.0.0 "" "/v2" \
  "$api_removed" "$internal" 0 "$(ran major %M@v1.0.0 v2.0.0)"
check "major on top of a v2 tag compares against the v2 module" '["major"]' v2.3.4 "/v2" "/v3" \
  "$api_removed" "$internal" 0 "$(ran major %M/v2@v2.3.4 v3.0.0)"
check "major without the new module path is a failure no label fixes" '["major"]' v1.0.0 "" "" \
  "$api_removed" "$internal" 1 "$failed"
check "patch that does not compile is a failure no label fixes" '["patch"]' v1.0.0 "" "" \
  "$api_broken" "$internal" 1 "$failed"
check "a missing release tag fails" '["patch"]' "" "" "" \
  "$api_added" "$internal" 1 "gorelease: no v* release tag found. Check out with fetch-depth: 0 so the tags are there."
check "patch with only unexported code changed passes" '["patch"]' v1.0.0 "" "" \
  "$api_hidden" "$internal" 0 "$(ran patch v1.0.0 v1.0.1)"
check "patch with only internal code changed passes" '["patch"]' v1.0.0 "" "" \
  "$api" "$internal_added" 0 "$(ran patch v1.0.0 v1.0.1)"
check "minor with only internal code changed passes" '["minor"]' v1.0.0 "" "" \
  "$api" "$internal_added" 0 "$(ran minor v1.0.0 v1.1.0)"
check "major with only internal code changed passes" '["major"]' v1.0.0 "" "/v2" \
  "$api" "$internal_added" 0 "$(ran major %M@v1.0.0 v2.0.0)"
check "no release label is left to the release label check" '[]' v1.0.0 "" "" \
  "$api_removed" "$internal" 0 "$skipped"
check "two release labels are left to the release label check" '["patch","minor"]' v1.0.0 "" "" \
  "$api_removed" "$internal" 0 "$skipped"

echo "$cases cases, $failures failed"
[ "$failures" -eq 0 ]
