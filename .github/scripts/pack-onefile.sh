#!/usr/bin/env bash
# Zip a staged runtime and embed it in one executable that does not need Qt installed.
set -euo pipefail

if [[ $# -ne 2 ]]; then
  echo "usage: pack-onefile.sh STAGE OUT" >&2
  exit 2
fi

root=$(cd "$(dirname "$0")/../.." && pwd)
stage=$1
out=$2
if [[ $stage != /* ]]; then stage=$root/$stage; fi
if [[ $out != /* ]]; then out=$root/$out; fi

payload=$root/cmd/onefile/payload.zip
rm -f "$payload"
trap 'rm -f "$payload"' EXIT
( cd "$root" && CGO_ENABLED=0 go run ./cmd/packzip "$stage" "$payload" )

ldflags="-s -w"
case $out in
  *.exe) ldflags="-s -w -H windowsgui" ;;
esac
mkdir -p "$(dirname "$out")"
(
  cd "$root"
  CGO_ENABLED=0 go build -tags onefile -trimpath -ldflags "$ldflags" -o "$out" ./cmd/onefile
)
rm -f "$payload"
echo "wrote $out"
