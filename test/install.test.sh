#!/usr/bin/env bash
# shellcheck source=scripts/tests/lib.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib.sh"

releases="$sandbox/releases"
version=1.2.3
case $(uname -s) in Darwin) os=darwin ;; *) os=linux ;; esac
case $(uname -m) in arm64 | aarch64) arch=arm64 ;; *) arch=amd64 ;; esac
archive="pik_${version}_${os}_${arch}.tar.gz"

mkdir -p "$releases/download/v$version" "$sandbox/build" "$sandbox/bin"
printf '#!/bin/sh\necho "pik version %s"\n' "$version" >"$sandbox/build/pik"
chmod +x "$sandbox/build/pik"
tar -czf "$releases/download/v$version/$archive" -C "$sandbox/build" pik
(cd "$releases/download/v$version" && shasum -a 256 "$archive" >checksums.txt)

for tool in awk cat chmod curl cut dirname gzip mkdir mktemp mv rm sed sh shasum tar tr uname; do
  ln -s "$(command -v "$tool")" "$sandbox/bin/$tool"
done

run_install() {
  env -i HOME="$sandbox/home" PATH="$sandbox/bin" PIK_DOWNLOAD_URL="file://$releases" \
    PIK_INSTALL_DIR="$sandbox/home/bin" "$@" sh "$root/install.sh" 2>&1
}

output=$(run_install PIK_VERSION="$version")
[[ $("$sandbox/home/bin/pik") == "pik version $version" ]] || fail "pik not installed: $output"
[[ $output == *"only the checksum is verified"* ]] || fail "should say the signature wasn't checked: $output"
[[ $output == *"Add $sandbox/home/bin to your PATH"* ]] || fail "should give a PATH hint: $output"
pass 'installs a pinned version and verifies its checksum'

output=$(run_install PIK_VERSION="v$version")
[[ $output == *"Installed pik $version"* ]] || fail "a v-prefixed version failed: $output"
pass 'accepts a version with a v prefix'

rm "$sandbox/home/bin/pik"
printf 'tampered' >>"$releases/download/v$version/$archive"
if output=$(run_install PIK_VERSION="$version"); then
  fail "a tampered archive was installed: $output"
fi
[[ $output == *"doesn't match, so nothing was installed"* ]] || fail "got: $output"
[[ ! -e $sandbox/home/bin/pik ]] || fail 'a tampered binary was left behind'
pass 'refuses an archive whose checksum does not match'

if output=$(run_install PIK_VERSION=9.9.9); then
  fail "a missing release succeeded: $output"
fi
[[ $output == *"couldn't download"* ]] || fail "got: $output"
pass 'fails clearly when the release does not exist'

rm "$sandbox/bin/tar"
if output=$(run_install PIK_VERSION="$version"); then
  fail 'ran without tar'
fi
[[ $output == *"tar is required"* ]] || fail "got: $output"
pass 'names a missing tool'
