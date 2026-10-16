#!/usr/bin/env bash
# Runs stbox once per feature that depends on how it was built: the version stamp and the code page tables.
# Usage: .build/smoke-test.sh <expected version> <testdata dir as stbox sees it> <stbox command...>
set -euo pipefail

expected=$1
files=$2
shift 2

version=$("$@" --version)
if [ "$version" != "$expected" ]; then
  echo "--version printed '$version', expected '$expected'" >&2
  exit 1
fi

vtt=$("$@" convert "$files/cli/latin1.srt" --encoding Windows-1252 --to vtt --output -)
if ! grep -qx 'Café au lait.' <<< "$vtt"; then
  printf 'convert printed:\n%s\n' "$vtt" >&2
  exit 1
fi

echo "smoke test passed: $version"
