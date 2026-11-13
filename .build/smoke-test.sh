#!/usr/bin/env bash
# Runs stbox once per feature that depends on how it was built: the version stamp, the code page tables, the ARIB tables and the glyph OCR engine.
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

srt=$("$@" convert "$files/mpegts/arib_japanese.ts" --track 304 --to srt --output -)
if ! grep -q "$(printf '\xe5\xa4\xa9\xe6\xb0\x97')" <<< "$srt"; then
  printf 'convert printed:\n%s\n' "$srt" >&2
  exit 1
fi

srt=$("$@" convert "$files/pgs/text_1080p.sup" --ocr --to srt --output -)
if ! grep -q 'Bergen' <<< "$srt"; then
  printf 'convert --ocr printed:\n%s\n' "$srt" >&2
  exit 1
fi

echo "smoke test passed: $version"
