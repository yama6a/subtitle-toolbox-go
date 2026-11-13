#!/usr/bin/env bash
# Runs stbox once per feature that depends on how it was built: the version stamp, the code page tables and the glyph OCR engine.
# With SMOKE_TESSERACT=1 it also reads a PGS file with Tesseract.
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

srt=$("$@" convert "$files/pgs/text_1080p.sup" --ocr --to srt --output -)
if ! grep -q 'Bergen' <<< "$srt"; then
  printf 'convert --ocr printed:\n%s\n' "$srt" >&2
  exit 1
fi

if [ "${SMOKE_TESSERACT:-}" = 1 ]; then
  srt=$("$@" convert "$files/pgs/text_cyrillic_1080p.sup" --ocr --ocr-engine tesseract --ocr-language rus --to srt --output -)
  word=$(printf '\xd0\x91\xd0\xb5\xd1\x80\xd0\xb3\xd0\xb5\xd0\xbd')
  if ! grep -q "$word" <<< "$srt"; then
    printf 'convert --ocr printed:\n%s\n' "$srt" >&2
    exit 1
  fi
fi

echo "smoke test passed: $version"
