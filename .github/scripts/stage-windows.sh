#!/usr/bin/env bash
# Build with the UCRT64 gcc that ships beside MSYS2 Qt 6, then let
# windeployqt drop the Qt DLLs and QML plugins next to the exe.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"

export CGO_ENABLED=1
export CC="${CC:-gcc}"
export CXX="${CXX:-g++}"
export CGO_CFLAGS="${CGO_CFLAGS:--O0}"
export CGO_CXXFLAGS="${CGO_CXXFLAGS:--O0}"
unset GOFLAGS

# setup-go puts go.exe on the Windows PATH. MSYS2 does not keep that entry,
# and it also uses a different HOME, so pin the module cache to the Windows
# user profile that the download step already filled.
if [[ -n ${USERPROFILE:-} ]]; then
  export GOPATH="$(cygpath -u "$USERPROFILE")/go"
  export GOCACHE="$(cygpath -u "${LOCALAPPDATA:-$USERPROFILE/AppData/Local}")/go-build"
  export GOMODCACHE="$GOPATH/pkg/mod"
fi
if ! command -v go >/dev/null 2>&1; then
  win=$(cmd.exe //c "where.exe go" 2>/dev/null | tr -d '\r' | head -n 1 || true)
  if [[ -n ${win:-} ]]; then
    unix=$(cygpath -u "$win")
    export PATH="$(dirname "$unix"):$PATH"
  fi
fi
if ! command -v go >/dev/null 2>&1; then
  shopt -s nullglob
  for c in /c/hostedtoolcache/windows/go/*/x64/bin /d/hostedtoolcache/windows/go/*/x64/bin; do
    if [[ -x $c/go.exe ]]; then
      export PATH="$c:$PATH"
      break
    fi
  done
fi
command -v go
go version
pkg-config --exists Qt6Widgets Qt6Qml
pkg-config --modversion Qt6Core Qt6Qml

stage=dist/windows
rm -rf "$stage"
mkdir -p "$stage/assets"

echo "== cli $(date -u +%H:%M:%S) =="
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$stage/mclauncher-cli.exe" ./cmd/mclauncher-cli
jobs=$(nproc 2>/dev/null || echo 4)
echo "== ui $(date -u +%H:%M:%S) jobs=$jobs =="
go build -trimpath -v -p "$jobs" -ldflags "-s -w -H windowsgui" -o "$stage/mclauncher.exe" ./cmd/mclauncher
echo "== ui done $(date -u +%H:%M:%S) =="

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
bash .github/scripts/pack-onefile.sh "$stage" dist/mclauncher-windows-amd64.exe
