#!/bin/sh
# Installs the latest rytrix-ddns release for macOS or Linux.
#   curl -fsSL https://raw.githubusercontent.com/ryansonshine/rytrix-ddns-client/main/install.sh | sh
set -eu

repo="ryansonshine/rytrix-ddns-client"

case "$(uname -s)" in
  Darwin) os=darwin ;;
  Linux) os=linux ;;
  *) echo "Unsupported OS: $(uname -s). On Windows, use install.ps1." >&2; exit 1 ;;
esac

case "$(uname -m)" in
  x86_64 | amd64) arch=amd64 ;;
  arm64 | aarch64) arch=arm64 ;;
  *) echo "Unsupported CPU: $(uname -m)" >&2; exit 1 ;;
esac

name="rytrix-ddns_${os}_${arch}.tar.gz"
base="https://github.com/${repo}/releases/latest/download"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "Downloading ${name}..."
curl -fsSL -o "$tmp/$name" "$base/$name"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"

expected="$(grep " ${name}\$" "$tmp/checksums.txt" | cut -d' ' -f1)"
if command -v sha256sum >/dev/null 2>&1; then
  actual="$(sha256sum "$tmp/$name" | cut -d' ' -f1)"
else
  actual="$(shasum -a 256 "$tmp/$name" | cut -d' ' -f1)"
fi
if [ -z "$expected" ] || [ "$expected" != "$actual" ]; then
  echo "Checksum mismatch for ${name}; not installing." >&2
  exit 1
fi

tar -xzf "$tmp/$name" -C "$tmp" rytrix-ddns

# /usr/local/bin when it's writable or sudo is available, otherwise ~/.local/bin.
dest="/usr/local/bin"
if [ -w "$dest" ]; then
  install -m 0755 "$tmp/rytrix-ddns" "$dest/rytrix-ddns"
elif command -v sudo >/dev/null 2>&1; then
  echo "Installing to $dest (sudo will ask for your password)..."
  sudo install -m 0755 "$tmp/rytrix-ddns" "$dest/rytrix-ddns"
else
  dest="$HOME/.local/bin"
  mkdir -p "$dest"
  install -m 0755 "$tmp/rytrix-ddns" "$dest/rytrix-ddns"
  case ":$PATH:" in *":$dest:"*) ;; *) echo "Add $dest to your PATH to run rytrix-ddns." ;; esac
fi

echo "Installed $("$dest/rytrix-ddns" version) to $dest."
echo "Next: rytrix-ddns setup"
