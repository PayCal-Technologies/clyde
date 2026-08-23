#!/usr/bin/env bash
set -euo pipefail

version="$(cat VERSION)"
host_os="$(uname -s | tr '[:upper:]' '[:lower:]')"
host_arch="$(uname -m)"

sha256_check() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum -c SHA256SUMS
  else
    shasum -a 256 -c SHA256SUMS
  fi
}

(cd dist && sha256_check)

for archive in dist/*.tar.gz; do
  name="$(basename "$archive" .tar.gz)"
  rm -rf /tmp/clyde-release-smoke
  mkdir -p /tmp/clyde-release-smoke
  tar -C /tmp/clyde-release-smoke -xzf "$archive"
  test -f "/tmp/clyde-release-smoke/${name}/README.md"
  test -f "/tmp/clyde-release-smoke/${name}/LICENSE"
  test -f "/tmp/clyde-release-smoke/${name}/VERSION"
  test -x "/tmp/clyde-release-smoke/${name}/clyde"
  if [[ "$name" == "clyde_${version}_linux_amd64" && "$host_os" == linux* && "$host_arch" == "x86_64" ]]; then
    "/tmp/clyde-release-smoke/${name}/clyde" --about
  fi
done

for archive in dist/*.zip; do
  name="$(basename "$archive" .zip)"
  rm -rf /tmp/clyde-release-smoke
  mkdir -p /tmp/clyde-release-smoke
  unzip -q "$archive" -d /tmp/clyde-release-smoke
  test -f "/tmp/clyde-release-smoke/${name}/README.md"
  test -f "/tmp/clyde-release-smoke/${name}/LICENSE"
  test -f "/tmp/clyde-release-smoke/${name}/VERSION"
  test -f "/tmp/clyde-release-smoke/${name}/clyde.exe"
done
