#!/usr/bin/env bash
set -euo pipefail

if [ ! -f go.mod ]; then
  echo "go.mod not found; run from repository root" >&2
  exit 1
fi

version="$(awk '$1 == "go" { print $2 }' go.mod)"
if [ -z "$version" ]; then
  echo "go.mod does not declare a Go version" >&2
  exit 1
fi

os_name="${1:-$(uname -s)}"
arch_name="${2:-$(uname -m)}"

case "$(printf '%s' "$os_name" | tr '[:upper:]' '[:lower:]')" in
  linux*) goos="linux" ;;
  darwin*) goos="darwin" ;;
  mingw*|msys*|cygwin*|windows*) goos="windows" ;;
  *) echo "unsupported OS: $os_name" >&2; exit 1 ;;
esac

case "$(printf '%s' "$arch_name" | tr '[:upper:]' '[:lower:]')" in
  x86_64|amd64) goarch="amd64" ;;
  arm64|aarch64) goarch="arm64" ;;
  *) echo "unsupported architecture: $arch_name" >&2; exit 1 ;;
esac

install_root="${RUNNER_TEMP:-/tmp}/go"
rm -rf "$install_root"

if [ "$goos" = "windows" ]; then
  archive="${RUNNER_TEMP:-/tmp}/go.zip"
  curl -fsSLo "$archive" "https://go.dev/dl/go${version}.${goos}-${goarch}.zip"
  unzip -q "$archive" -d "${RUNNER_TEMP:-/tmp}"
else
  archive="${RUNNER_TEMP:-/tmp}/go.tar.gz"
  curl -fsSLo "$archive" "https://go.dev/dl/go${version}.${goos}-${goarch}.tar.gz"
  tar -C "${RUNNER_TEMP:-/tmp}" -xzf "$archive"
fi

if [ -n "${GITHUB_PATH:-}" ]; then
  echo "${install_root}/bin" >> "$GITHUB_PATH"
fi

"${install_root}/bin/go" version
