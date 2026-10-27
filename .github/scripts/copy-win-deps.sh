#!/usr/bin/env bash
# windeployqt does not copy the UCRT64 runtime Qt was linked against
# (libstdc++, ICU, pcre2, ...). Walk every exe and dll and copy those in,
# or the program will not start on a machine without MSYS2.
set -euo pipefail

stage=${1:?stage dir}
search=${2:-/ucrt64/bin}
objdump=${OBJDUMP:-objdump}

have() {
  local dll=$1
  find "$stage" -iname "$dll" -print -quit | grep -q .
}

find_src() {
  find "$search" -iname "$1" -print -quit
}

round=0
while true; do
  round=$((round + 1))
  added=0
  while IFS= read -r bin; do
    while IFS= read -r dll; do
      [[ -n $dll ]] || continue
      if have "$dll"; then
        continue
      fi
      src=$(find_src "$dll" || true)
      if [[ -z $src ]]; then
        continue
      fi
      cp -L "$src" "$stage/$dll"
      echo "runtime $dll"
      added=1
    done < <("$objdump" -p "$bin" 2>/dev/null | awk '/DLL Name:/{print $3}')
  done < <(find "$stage" -type f \( -iname '*.dll' -o -iname '*.exe' \))
  if [[ $added -eq 0 ]]; then
    break
  fi
  if [[ $round -gt 8 ]]; then
    echo "runtime copy did not settle" >&2
    exit 1
  fi
done

missing=0
while IFS= read -r bin; do
  while IFS= read -r dll; do
    [[ -n $dll ]] || continue
    if have "$dll"; then
      continue
    fi
    src=$(find_src "$dll" || true)
    if [[ -n $src ]]; then
      echo "still missing $dll (from $(basename "$bin"))" >&2
      missing=1
    fi
  done < <("$objdump" -p "$bin" 2>/dev/null | awk '/DLL Name:/{print $3}')
done < <(find "$stage" -type f \( -iname '*.dll' -o -iname '*.exe' \))
if [[ $missing -ne 0 ]]; then
  exit 1
fi

# Plugins sit next to the exe (platforms/, qml/), not in the MSYS2 prefix.
cat > "$stage/qt.conf" <<'EOF'
[Paths]
Prefix=.
Plugins=.
QmlImports=qml
Qml2Imports=qml
EOF
echo "wrote qt.conf"
