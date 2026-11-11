#!/usr/bin/env bash
# Build release archives and checksums into ./dist
#
#   scripts/build-release.sh v0.1.0
#
# Produces gclone_<version>_<os>_<arch>.tar.gz (zip on Windows) and checksums.txt.
set -euo pipefail

VERSION_TAG="${1:?usage: scripts/build-release.sh <version, e.g. v0.1.0>}"
VERSION="${VERSION_TAG#v}"
OUT="dist"
TARGETS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"

cd "$(dirname "$0")/.."
rm -rf "$OUT"
mkdir -p "$OUT"

for target in $TARGETS; do
  os="${target%/*}"
  arch="${target#*/}"
  name="gclone_${VERSION}_${os}_${arch}"
  bin="gclone"
  [ "$os" = "windows" ] && bin="gclone.exe"

  mkdir -p "$OUT/$name"
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags "-s -w -X main.version=${VERSION}" -o "$OUT/$name/$bin" .
  cp LICENSE README.md "$OUT/$name/"

  if [ "$os" = "windows" ]; then
    (cd "$OUT" && zip -qr "$name.zip" "$name")
  else
    tar -C "$OUT" -czf "$OUT/$name.tar.gz" "$name"
  fi
  rm -rf "$OUT/$name"
done

(cd "$OUT" && sha256sum gclone_* > checksums.txt)
echo "Built:"
ls -1 "$OUT"
