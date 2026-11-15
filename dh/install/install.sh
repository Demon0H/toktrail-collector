#!/bin/sh
set -eu
case "$(uname -s)" in Darwin) os=darwin;; Linux) os=linux;; *) echo 'Use install.ps1 on Windows' >&2; exit 1;; esac
case "$(uname -m)" in x86_64) arch=amd64;; arm64|aarch64) arch=arm64;; *) echo 'Unsupported architecture' >&2; exit 1;; esac
base=https://github.com/Demon0H/toktrail-collector/releases/latest/download
asset="dh-tokenrank-$os-$arch.tar.gz"
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT HUP INT TERM
curl --fail --location --proto '=https' --tlsv1.2 "$base/$asset" -o "$tmp/$asset"
curl --fail --location --proto '=https' --tlsv1.2 "$base/SHA256SUMS" -o "$tmp/SHA256SUMS"
cd "$tmp"
grep "  $asset$" SHA256SUMS > selected.sha256
if command -v sha256sum >/dev/null 2>&1; then sha256sum -c selected.sha256; else shasum -a 256 -c selected.sha256; fi
tar xzf "$asset"
dest="$HOME/.local/share/dh-tokenrank"
mkdir -p "$dest"
cp "dh-tokenrank-$os-$arch/dh-tokenrank" "dh-tokenrank-$os-$arch/toktrail" "$dest/"
chmod 700 "$dest/dh-tokenrank" "$dest/toktrail"
printf 'Installed: %s/dh-tokenrank\nNext: read AGENT_SETUP.md and explicitly authorize sources.\n' "$dest"
