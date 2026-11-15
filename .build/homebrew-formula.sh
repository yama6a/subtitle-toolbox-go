#!/usr/bin/env bash
# Writes the Homebrew formula of stbox for one release to stdout.
# It takes the four darwin and linux archives and their SHA-256 sums from the checksum file of the release.
# Usage: .build/homebrew-formula.sh <version> <checksum file>
set -euo pipefail

version=${1:-}
sums=${2:-}

fail() {
  echo "$1" >&2
  exit 1
}

if [ -z "$version" ]; then
  fail "usage: $0 <version> <checksum file>. The version is empty."
fi
if [ ! -f "$sums" ]; then
  fail "usage: $0 <version> <checksum file>. The checksum file \"$sums\" does not exist."
fi

base="https://github.com/yama6a/subtitle-toolbox-go/releases/download/v$version"

sum() {
  local name="stbox_${version}_$1.tar.gz" found
  found=$(awk -v n="$name" '$2 == n { print $1 }' "$sums")
  if ! [[ $found =~ ^[0-9a-f]{64}$ ]]; then
    fail "The checksum file has no SHA-256 sum for $name."
  fi
  echo "$found"
}

darwin_arm64=$(sum darwin_arm64)
darwin_amd64=$(sum darwin_amd64)
linux_arm64=$(sum linux_arm64)
linux_amd64=$(sum linux_amd64)

cat << EOF
# Written by .build/homebrew-formula.sh in yama6a/subtitle-toolbox-go on each release.
class Stbox < Formula
  desc "Read, edit and write subtitle files"
  homepage "https://github.com/yama6a/subtitle-toolbox-go"
  license "MIT"

  on_macos do
    on_arm do
      url "$base/stbox_${version}_darwin_arm64.tar.gz"
      sha256 "$darwin_arm64"
    end
    on_intel do
      url "$base/stbox_${version}_darwin_amd64.tar.gz"
      sha256 "$darwin_amd64"
    end
  end

  on_linux do
    on_arm do
      url "$base/stbox_${version}_linux_arm64.tar.gz"
      sha256 "$linux_arm64"
    end
    on_intel do
      url "$base/stbox_${version}_linux_amd64.tar.gz"
      sha256 "$linux_amd64"
    end
  end

  def install
    bin.install "stbox"
    prefix.install "THIRD_PARTY_NOTICES.md"
  end

  def caveats
    <<~TEXT
      stbox reads cue images with its built-in glyph OCR engine.
      To use the Tesseract engine instead, install Tesseract too:
        brew install tesseract
    TEXT
  end

  test do
    assert_equal version.to_s, shell_output("#{bin}/stbox --version").strip
  end
end
EOF
