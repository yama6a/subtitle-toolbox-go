#!/usr/bin/env bash
# Builds the release archives of stbox for 6 targets and their checksum file.
# A rebuild of the same commit gives the same bytes.
# Usage: .build/build-release.sh <version> <output dir>
set -euo pipefail

version=${1:-}
out=${2:-}

fail() {
  echo "$1" >&2
  exit 1
}

if [ -z "$version" ]; then
  fail "usage: $0 <version> <output dir>. The version is empty."
fi
if [ -z "$out" ]; then
  fail "usage: $0 <version> <output dir>. The output dir is empty."
fi
if [ -d "$out" ] && [ -n "$(ls -A "$out")" ]; then
  fail "The output dir $out already holds files. Remove them first."
fi

mkdir -p "$out"
out=$(cd "$out" && pwd)
root=$(git rev-parse --show-toplevel)
cd "$root"
mtime=$(git log -1 --format=%ct)
export LC_ALL=C
# zip stores local time, so a fixed zone keeps the archive bytes the same on every machine.
export TZ=UTC

stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT

docs=(LICENSE README.md THIRD_PARTY_NOTICES.md)
# An archive holds no docs/ folder, so its README links the docs pages at the tag.
sed "s#](docs/#](https://github.com/yama6a/subtitle-toolbox-go/blob/v$version/docs/#g" README.md > "$stage/README.md"
for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64; do
  goos=${target%/*}
  goarch=${target#*/}
  name="stbox_${version}_${goos}_${goarch}"
  binary=stbox
  if [ "$goos" = windows ]; then binary=stbox.exe; fi

  dir="$stage/$name"
  mkdir -p "$dir"
  CGO_ENABLED=0 GOOS=$goos GOARCH=$goarch go build -trimpath \
    -ldflags "-s -w -X main.version=$version" -o "$dir/$binary" ./cmd/stbox
  cp LICENSE THIRD_PARTY_NOTICES.md "$stage/README.md" "$dir/"
  # Fixed modes and times, so neither the umask nor the checkout time changes the archive bytes.
  chmod 0644 "$dir"/*
  chmod 0755 "$dir/$binary"
  touch -d "@$mtime" "$dir"/*

  if [ "$goos" = windows ]; then
    (cd "$dir" && zip -q -X "$out/$name.zip" "${docs[@]}" "$binary")
  else
    tar -C "$dir" --sort=name --owner=0 --group=0 --numeric-owner --mtime="@$mtime" -cf - "${docs[@]}" "$binary" \
      | gzip -n -9 > "$out/$name.tar.gz"
  fi
done

cd "$out"
mapfile -t archives < <(printf '%s\n' *.tar.gz *.zip | sort)
sha256sum "${archives[@]}" > "stbox_${version}_checksums.txt"

for file in *; do
  printf '%s %s bytes\n' "$file" "$(wc -c < "$file")"
done
