#!/bin/sh
set -eu
case "$(uname -s)" in Darwin) os=darwin;; Linux) os=linux;; *) echo 'Use install.ps1 on Windows' >&2; exit 1;; esac
case "$(uname -m)" in x86_64) arch=amd64;; arm64|aarch64) arch=arm64;; *) echo 'Unsupported architecture' >&2; exit 1;; esac
download() {
  if command -v curl >/dev/null 2>&1 && curl --fail --location --proto '=https' --proto-redir '=https' --tlsv1.2 --connect-timeout 20 --max-time 180 "$1" -o "$2"; then return; fi
  if command -v wget >/dev/null 2>&1 && wget --https-only --timeout=180 -O "$2" "$1"; then return; fi
  if command -v python3 >/dev/null 2>&1 && python3 -c 'import sys,urllib.request; r=urllib.request.urlopen(sys.argv[1],timeout=180); data=r.read(); open(sys.argv[2],"wb").write(data)' "$1" "$2"; then return; fi
  echo 'Verified downloads failed. Download archive and SHA256SUMS from the official GitHub Release in a browser, verify SHA-256, then extract. Never disable TLS verification.' >&2
  return 1
}
base=https://github.com/Demon0H/toktrail-collector/releases/latest/download
asset="dh-tokenrank-$os-$arch.tar.gz"
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT HUP INT TERM
download "$base/$asset" "$tmp/$asset"
download "$base/SHA256SUMS" "$tmp/SHA256SUMS"
cd "$tmp"
awk -v name="$asset" '$2 == name {print; count++} END {if (count != 1) exit 1}' SHA256SUMS > selected.sha256
if command -v sha256sum >/dev/null 2>&1; then sha256sum -c selected.sha256; else shasum -a 256 -c selected.sha256; fi
tar xzf "$asset"
dest="$HOME/.local/share/dh-tokenrank"
mkdir -p "$dest"
cp "dh-tokenrank-$os-$arch/dh-tokenrank" "dh-tokenrank-$os-$arch/toktrail" "$dest/"
chmod 700 "$dest/dh-tokenrank" "$dest/toktrail"
printf 'Installed: %s/dh-tokenrank\nNext: read AGENT_SETUP.md and explicitly authorize sources.\n' "$dest"
