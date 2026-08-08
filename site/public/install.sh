#!/bin/sh
# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# okfctl installer.
#
# Detects your OS and architecture, downloads the matching release archive from
# GitHub, VERIFIES it against the release's published checksums.txt, and installs
# both `okfctl` and its bundled `okfctl-search` plugin onto your PATH.
#
# Usage:
#   curl -sSL https://okfctl.dev/install.sh | sh
#
# Environment overrides:
#   OKFCTL_VERSION   install a specific tag (e.g. v0.2.0). Default: latest.
#   OKFCTL_BIN_DIR   install directory. Default: /usr/local/bin (falls back to
#                    $HOME/.local/bin when /usr/local/bin is not writable).
#
# The install ABORTS on any checksum mismatch — a corrupted or tampered archive
# is never installed.
set -eu

REPO="cwest/okfctl"
BIN_DIR="${OKFCTL_BIN_DIR:-/usr/local/bin}"

info() { printf '\033[0;34m==>\033[0m %s\n' "$1" >&2; }
err() { printf '\033[0;31merror:\033[0m %s\n' "$1" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || err "required command not found: $1"
}

# --- prerequisites -----------------------------------------------------------
# A downloader (curl or wget) and a SHA-256 tool are mandatory; without the
# latter we cannot verify the archive, and installing unverified is not allowed.
if command -v curl >/dev/null 2>&1; then
  DL="curl -fsSL"
  DL_O="curl -fsSL -o"
elif command -v wget >/dev/null 2>&1; then
  DL="wget -qO-"
  DL_O="wget -qO"
else
  err "need curl or wget to download okfctl"
fi

if command -v sha256sum >/dev/null 2>&1; then
  sha256() { sha256sum "$1" | awk '{print $1}'; }
elif command -v shasum >/dev/null 2>&1; then
  sha256() { shasum -a 256 "$1" | awk '{print $1}'; }
else
  err "need sha256sum or shasum to verify the download; refusing to install unverified"
fi

need tar
need uname
need mktemp

# --- detect OS / arch --------------------------------------------------------
os="$(uname -s)"
case "$os" in
  Darwin) os="darwin" ;;
  Linux)  os="linux" ;;
  *) err "unsupported OS: $os (this installer supports macOS and Linux; on Windows download the .zip from the releases page)" ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64 | amd64) arch="amd64" ;;
  arm64 | aarch64) arch="arm64" ;;
  *) err "unsupported architecture: $arch" ;;
esac

# --- resolve version ---------------------------------------------------------
version="${OKFCTL_VERSION:-}"
if [ -z "$version" ]; then
  info "resolving latest release"
  # Follow the /releases/latest redirect to learn the tag without a GitHub token.
  version="$(
    $DL "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep -m1 '"tag_name"' \
      | sed -E 's/.*"tag_name" *: *"([^"]+)".*/\1/'
  )"
  [ -n "$version" ] || err "could not determine the latest version; set OKFCTL_VERSION and retry"
fi
info "installing okfctl ${version} for ${os}/${arch}"

# Archive name matches .goreleaser.yaml name_template:
#   okfctl_<version-without-leading-v>_<os>_<arch>.tar.gz
ver_no_v="${version#v}"
archive="okfctl_${ver_no_v}_${os}_${arch}.tar.gz"
base="https://github.com/${REPO}/releases/download/${version}"

# --- download ----------------------------------------------------------------
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT INT TERM

info "downloading ${archive}"
$DL_O "${tmp}/${archive}" "${base}/${archive}" \
  || err "failed to download ${base}/${archive}"

info "downloading checksums.txt"
$DL_O "${tmp}/checksums.txt" "${base}/checksums.txt" \
  || err "failed to download ${base}/checksums.txt"

# --- verify checksum ---------------------------------------------------------
want="$(grep " ${archive}\$" "${tmp}/checksums.txt" | awk '{print $1}' | head -n1)"
[ -n "$want" ] || err "no checksum for ${archive} in checksums.txt"

got="$(sha256 "${tmp}/${archive}")"
if [ "$want" != "$got" ]; then
  err "checksum mismatch for ${archive}
  expected: ${want}
  actual:   ${got}
refusing to install a corrupted or tampered archive"
fi
info "checksum verified"

# --- extract & install -------------------------------------------------------
tar -xzf "${tmp}/${archive}" -C "$tmp" \
  || err "failed to extract ${archive}"

for bin in okfctl okfctl-search; do
  [ -f "${tmp}/${bin}" ] || err "expected binary ${bin} missing from archive"
  chmod +x "${tmp}/${bin}"
done

install_one() {
  # $1 = binary name
  src="${tmp}/$1"
  dst="${BIN_DIR}/$1"
  if [ -w "$BIN_DIR" ] || { [ ! -e "$BIN_DIR" ] && [ -w "$(dirname "$BIN_DIR")" ]; }; then
    mkdir -p "$BIN_DIR"
    mv "$src" "$dst"
  elif command -v sudo >/dev/null 2>&1; then
    info "elevating with sudo to write ${dst}"
    sudo mkdir -p "$BIN_DIR"
    sudo mv "$src" "$dst"
  else
    err "cannot write to ${BIN_DIR} and sudo is unavailable; set OKFCTL_BIN_DIR to a writable directory"
  fi
}

# If /usr/local/bin is not writable and there's no sudo, fall back to a
# per-user dir before failing.
if [ "$BIN_DIR" = "/usr/local/bin" ] && [ ! -w "$BIN_DIR" ] && ! command -v sudo >/dev/null 2>&1; then
  BIN_DIR="${HOME}/.local/bin"
  info "/usr/local/bin not writable; installing to ${BIN_DIR}"
fi

install_one okfctl
install_one okfctl-search

info "installed okfctl and okfctl-search to ${BIN_DIR}"
case ":${PATH}:" in
  *":${BIN_DIR}:"*) : ;;
  *) info "note: ${BIN_DIR} is not on your PATH; add it to use okfctl" ;;
esac

if command -v okfctl >/dev/null 2>&1; then
  okfctl version || true
else
  info "run: ${BIN_DIR}/okfctl version"
fi
