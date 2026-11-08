#!/bin/sh
# Installs pik: curl -fsSL https://raw.githubusercontent.com/opikdev/starters/main/install.sh | sh
set -eu

repo=opikdev/starters
releases=${PIK_DOWNLOAD_URL:-https://github.com/$repo/releases}
install_dir=${PIK_INSTALL_DIR:-$HOME/.local/bin}

fail() {
  echo "pik install: $*" >&2
  exit 1
}

need() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required. Install it, then rerun this script."
}

need curl
need tar
need uname

case $(uname -s) in
  Darwin) os=darwin ;;
  Linux)
    os=linux
    need gzip
    ;;
  *) fail "$(uname -s) isn't supported by this script. On Windows, run: scoop bucket add opikdev https://github.com/opikdev/scoop-bucket; scoop install pik" ;;
esac

case $(uname -m) in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) fail "$(uname -m) CPUs aren't supported" ;;
esac

if [ -n "${PIK_VERSION:-}" ]; then
  tag=v${PIK_VERSION#v}
else
  tag=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "$releases/latest" | sed 's#.*/tag/##')
  case $tag in
    v[0-9]*) ;;
    *) fail "couldn't find the latest release at $releases" ;;
  esac
fi
version=${tag#v}
archive=pik_${version}_${os}_${arch}.tar.gz

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Downloading pik $version for $os/$arch"
curl -fsSL -o "$tmp/$archive" "$releases/download/$tag/$archive" || fail "couldn't download $archive"
curl -fsSL -o "$tmp/checksums.txt" "$releases/download/$tag/checksums.txt" || fail "couldn't download checksums.txt"

if command -v cosign >/dev/null 2>&1; then
  curl -fsSL -o "$tmp/checksums.txt.sigstore.json" "$releases/download/$tag/checksums.txt.sigstore.json" ||
    fail "couldn't download the signature"
  cosign verify-blob \
    --certificate-identity "https://github.com/$repo/.github/workflows/release.yml@refs/tags/$tag" \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com \
    --bundle "$tmp/checksums.txt.sigstore.json" "$tmp/checksums.txt" >/dev/null 2>&1 ||
    fail "the signature on checksums.txt is invalid, so nothing was installed"
  echo "Verified the release signature"
else
  echo "cosign isn't installed, so only the checksum is verified"
fi

expected=$(awk -v file="$archive" '$2 == file { print $1 }' "$tmp/checksums.txt")
[ -n "$expected" ] || fail "checksums.txt has no entry for $archive"
if command -v sha256sum >/dev/null 2>&1; then
  actual=$(sha256sum "$tmp/$archive" | cut -d' ' -f1)
else
  actual=$(shasum -a 256 "$tmp/$archive" | cut -d' ' -f1)
fi
[ "$expected" = "$actual" ] || fail "the checksum of $archive doesn't match, so nothing was installed"

tar -xzf "$tmp/$archive" -C "$tmp" pik
mkdir -p "$install_dir"
mv "$tmp/pik" "$install_dir/pik"
chmod +x "$install_dir/pik"
echo "Installed pik $version to $install_dir/pik"

case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) echo "Add $install_dir to your PATH to run pik" ;;
esac
