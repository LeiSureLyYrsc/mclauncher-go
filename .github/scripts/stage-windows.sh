#!/usr/bin/env bash
# Build with the UCRT64 gcc that ships beside MSYS2 Qt 6, then let
# windeployqt drop the Qt DLLs and QML plugins next to the exe.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"

export CGO_ENABLED=1
export CC="${CC:-gcc}"
export CXX="${CXX:-g++}"
export GOFLAGS="${GOFLAGS:--p=2}"

if ! command -v go >/dev/null 2>&1; then
  if [[ -x /c/Program\ Files/Go/bin/go.exe ]]; then
    export PATH="/c/Program Files/Go/bin:$PATH"
  fi
fi
go version
pkg-config --exists Qt6Widgets Qt6Qml
pkg-config --modversion Qt6Core Qt6Qml

stage=dist/windows
rm -rf "$stage"
mkdir -p "$stage/assets"

CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$stage/mclauncher-cli.exe" ./cmd/mclauncher-cli
go build -trimpath -ldflags "-s -w -H windowsgui" -o "$stage/mclauncher.exe" ./cmd/mclauncher

cp -a assets/qml assets/skins "$stage/assets/"

deploy=""
for name in windeployqt6 windeployqt; do
  if command -v "$name" >/dev/null 2>&1; then
    deploy=$name
    break
  fi
done
if [[ -z $deploy ]]; then
  echo "windeployqt not found" >&2
  exit 1
fi
"$deploy" --compiler-runtime --qmldir "$root/assets/qml" "$stage/mclauncher.exe"

mkdir -p dist
rm -f dist/mclauncher-windows-amd64.zip
(cd dist && tar -a -cf mclauncher-windows-amd64.zip windows)
echo "wrote dist/mclauncher-windows-amd64.zip"
