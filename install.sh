#!/usr/bin/env bash
# Installs or updates stbox. Usage: install.sh [vX.Y.Z]
set -euo pipefail

repo=yama6a/subtitle-toolbox-go
install_dir="$HOME/.stbox"
link=/usr/local/bin/stbox

die() {
  echo "error: $*" >&2
  exit 1
}

for tool in curl tar; do
  command -v "$tool" > /dev/null || die "$tool is not installed"
done

case "$(uname -s)" in
  Linux) os=linux ;;
  Darwin) os=darwin ;;
  *) die "unsupported OS $(uname -s), use Linux or macOS" ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) die "unsupported architecture $(uname -m)" ;;
esac

if [[ -n "${1:-}" ]]; then
  version="${1#v}"
else
  latest="$(curl -fsSL -o /dev/null -w '%{url_effective}' "https://github.com/$repo/releases/latest")" \
    || die "cannot find the latest release"
  version="${latest##*/v}"
fi
[[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "bad version '$version', expected X.Y.Z or vX.Y.Z"

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

archive="stbox_${version}_${os}_${arch}.tar.gz"
checksums="stbox_${version}_checksums.txt"
base="https://github.com/$repo/releases/download/v$version"
curl -fsSL -o "$tmp/$archive" "$base/$archive" || die "cannot download $base/$archive"
curl -fsSL -o "$tmp/$checksums" "$base/$checksums" || die "cannot download $base/$checksums"

expected="$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/$checksums")"
[[ -n "$expected" ]] || die "$checksums has no entry for $archive"
if command -v sha256sum > /dev/null; then
  actual="$(sha256sum "$tmp/$archive" | awk '{ print $1 }')"
else
  actual="$(shasum -a 256 "$tmp/$archive" | awk '{ print $1 }')"
fi
[[ "$actual" == "$expected" ]] || die "checksum mismatch for $archive"

mkdir -p "$install_dir"
tar -xzf "$tmp/$archive" -C "$install_dir" stbox

if [[ "$(readlink "$link" 2> /dev/null || true)" != "$install_dir/stbox" ]]; then
  sudo mkdir -p "$(dirname "$link")"
  sudo ln -sf "$install_dir/stbox" "$link"
fi

echo "stbox $version installed to $install_dir/stbox and linked at $link"
