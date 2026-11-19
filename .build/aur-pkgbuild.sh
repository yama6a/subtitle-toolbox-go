#!/usr/bin/env bash
# Writes PKGBUILD and .SRCINFO of the AUR package stbox-bin for one release into a folder.
# It takes the two linux archives and their SHA-256 sums from the checksum file of the release.
# Usage: .build/aur-pkgbuild.sh <version> <checksum file> <output dir>
set -euo pipefail

version=${1:-}
sums=${2:-}
out=${3:-}

fail() {
  echo "$1" >&2
  exit 1
}

if [ -z "$version" ]; then
  fail "usage: $0 <version> <checksum file> <output dir>. The version is empty."
fi
if [ ! -f "$sums" ]; then
  fail "usage: $0 <version> <checksum file> <output dir>. The checksum file \"$sums\" does not exist."
fi
if [ -z "$out" ]; then
  fail "usage: $0 <version> <checksum file> <output dir>. The output dir is empty."
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

linux_amd64=$(sum linux_amd64)
linux_arm64=$(sum linux_arm64)

mkdir -p "$out"

cat > "$out/PKGBUILD" << EOF
# Written by .build/aur-pkgbuild.sh in yama6a/subtitle-toolbox-go on each release.
pkgname=stbox-bin
pkgver=$version
pkgrel=1
pkgdesc='Read, edit and write subtitle files'
arch=('x86_64' 'aarch64')
url='https://github.com/yama6a/subtitle-toolbox-go'
license=('MIT')
provides=('stbox')
conflicts=('stbox')
optdepends=('tesseract: OCR of image subtitles')
source_x86_64=("stbox_\${pkgver}_linux_amd64.tar.gz::$base/stbox_\${pkgver}_linux_amd64.tar.gz")
source_aarch64=("stbox_\${pkgver}_linux_arm64.tar.gz::$base/stbox_\${pkgver}_linux_arm64.tar.gz")
sha256sums_x86_64=('$linux_amd64')
sha256sums_aarch64=('$linux_arm64')

package() {
  install -Dm755 stbox "\$pkgdir/usr/bin/stbox"
  install -Dm644 THIRD_PARTY_NOTICES.md "\$pkgdir/usr/share/licenses/\$pkgname/THIRD_PARTY_NOTICES.md"
  install -Dm644 LICENSE "\$pkgdir/usr/share/licenses/\$pkgname/LICENSE"
}
EOF

cat > "$out/.SRCINFO" << EOF
pkgbase = stbox-bin
	pkgdesc = Read, edit and write subtitle files
	pkgver = $version
	pkgrel = 1
	url = https://github.com/yama6a/subtitle-toolbox-go
	arch = x86_64
	arch = aarch64
	license = MIT
	optdepends = tesseract: OCR of image subtitles
	provides = stbox
	conflicts = stbox
	source_x86_64 = stbox_${version}_linux_amd64.tar.gz::$base/stbox_${version}_linux_amd64.tar.gz
	sha256sums_x86_64 = $linux_amd64
	source_aarch64 = stbox_${version}_linux_arm64.tar.gz::$base/stbox_${version}_linux_arm64.tar.gz
	sha256sums_aarch64 = $linux_arm64

pkgname = stbox-bin
EOF
