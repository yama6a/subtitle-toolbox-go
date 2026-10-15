#!/usr/bin/env bash
# Runs stbox once per feature that depends on how it was built: the version stamp.
# Usage: .build/smoke-test.sh <expected version> <testdata dir as stbox sees it> <stbox command...>
set -euo pipefail

expected=$1
# shellcheck disable=SC2034 # the convert checks of later issues read the fixtures from here
files=$2
shift 2

version=$("$@" --version)
if [ "$version" != "$expected" ]; then
  echo "--version printed '$version', expected '$expected'" >&2
  exit 1
fi

echo "smoke test passed: $version"
