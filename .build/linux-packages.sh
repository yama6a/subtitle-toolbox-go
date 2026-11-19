#!/usr/bin/env bash
# Builds the deb and rpm packages of stbox for linux/amd64 and linux/arm64 with nfpm.
# It reads the linux archives in the dist dir, so each package holds the binary of its archive.
# It writes the 4 packages to the dist dir and appends their SHA-256 sums to the checksum file.
# Usage: .build/linux-packages.sh <version> <dist dir>
set -euo pipefail

version=${1:-}
dist=${2:-}

fail() {
  echo "$1" >&2
  exit 1
}

if [ -z "$version" ]; then
  fail "usage: $0 <version> <dist dir>. The version is empty."
fi
if [ ! -d "$dist" ]; then
  fail "usage: $0 <version> <dist dir>. The dist dir \"$dist\" does not exist."
fi
if ! command -v nfpm > /dev/null; then
  fail "nfpm is not on the PATH."
fi

dist=$(cd "$dist" && pwd)
sums="$dist/stbox_${version}_checksums.txt"
if [ ! -f "$sums" ]; then
  fail "The checksum file $sums does not exist."
fi
root=$(git rev-parse --show-toplevel)
# nfpm stamps the package files with this time, so a rebuild of the same commit gives the same bytes.
SOURCE_DATE_EPOCH=$(git -C "$root" log -1 --format=%ct)
export SOURCE_DATE_EPOCH

stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT

packages=()
for arch in amd64 arm64; do
  rpm_arch=x86_64
  if [ "$arch" = arm64 ]; then rpm_arch=aarch64; fi

  rm -rf "${stage:?}/$arch"
  mkdir -p "$stage/$arch"
  tar -xzf "$dist/stbox_${version}_linux_$arch.tar.gz" -C "$stage/$arch" stbox LICENSE THIRD_PARTY_NOTICES.md

  # nfpm resolves the source paths of nfpm.yaml against its working directory.
  (
    cd "$stage/$arch"
    VERSION=$version ARCH=$arch nfpm package --config "$root/.build/nfpm.yaml" --packager deb --target "$dist/stbox_$arch.deb"
    VERSION=$version ARCH=$arch nfpm package --config "$root/.build/nfpm.yaml" --packager rpm --target "$dist/stbox.$rpm_arch.rpm"
  )
  packages+=("stbox_$arch.deb" "stbox.$rpm_arch.rpm")
done

(cd "$dist" && sha256sum "${packages[@]}" >> "$sums")
for package in "${packages[@]}"; do
  printf '%s %s bytes\n' "$package" "$(wc -c < "$dist/$package")"
done
