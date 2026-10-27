#!/usr/bin/env bash
# Build the CLI and the Qt Quick window, then stage a tarball that carries
# the QML assets and the Qt libraries it was linked against.
set -euo pipefail

root=$(cd "$(dirname "$0")/../.." && pwd)
cd "$root"

stage=dist/linux
rm -rf "$stage"
mkdir -p "$stage/lib" "$stage/plugins" "$stage/qml" "$stage/assets"

export CGO_ENABLED=0
echo "== cli $(date -u +%H:%M:%S) =="
go build -trimpath -ldflags "-s -w" -o "$stage/mclauncher-cli" ./cmd/mclauncher-cli

# miqt's Qt binding is hundreds of generated C++ files. -O2 makes that look
# stuck; -O0 is enough for a launcher and finishes on a 4-core runner.
export CGO_ENABLED=1
export CGO_CFLAGS="${CGO_CFLAGS:--O0}"
export CGO_CXXFLAGS="${CGO_CXXFLAGS:--O0}"
echo "== ui $(date -u +%H:%M:%S) jobs=$(nproc) =="
go build -trimpath -v -p "$(nproc)" \
  -ldflags '-s -w -extldflags "-Wl,-rpath,$ORIGIN/lib"' \
  -o "$stage/mclauncher" ./cmd/mclauncher
echo "== ui done $(date -u +%H:%M:%S) =="

cp -a assets/qml assets/skins "$stage/assets/"

skip_lib() {
  case "$1" in
    libc.so*|libm.so*|libdl.so*|libpthread.so*|librt.so*|libresolv.so*|ld-linux*)
      return 0
      ;;
  esac
  return 1
}

declare -A seen=()
queue=()
enqueue() {
  local so=$1
  [[ -f $so ]] || return 0
  local base
  base=$(basename "$so")
  skip_lib "$base" && return 0
  [[ -n ${seen[$base]:-} ]] && return 0
  seen[$base]=1
  queue+=("$so")
}

enqueue_deps() {
  local bin=$1
  local line path
  while read -r line; do
    path=${line#*=> }
    path=${path%% *}
    [[ $path == /* ]] || continue
    enqueue "$path"
  done < <(ldd "$bin" 2>/dev/null || true)
}

enqueue_deps "$stage/mclauncher"
qml_dir=$(qtpaths6 --query QT_INSTALL_QML)
plugin_dir=$(qtpaths6 --query QT_INSTALL_PLUGINS)
for mod in QtQml QtQuick; do
  if [[ -d $qml_dir/$mod ]]; then
  cp -aL "$qml_dir/$mod" "$stage/qml/"
  fi
done
for plug in platforms imageformats xcbglintegrations platforminputcontexts iconengines; do
  if [[ -d $plugin_dir/$plug ]]; then
    mkdir -p "$stage/plugins/$plug"
    cp -aL "$plugin_dir/$plug/." "$stage/plugins/$plug/"
  fi
done

shopt -s globstar nullglob
for so in "$stage/qml"/**/*.so "$stage/plugins"/**/*.so; do
  enqueue_deps "$so"
done

# Copy in waves so dependencies of newly copied libraries are included.
while ((${#queue[@]})); do
  so=${queue[0]}
  if ((${#queue[@]} == 1)); then
    queue=()
  else
    queue=("${queue[@]:1}")
  fi
  base=$(basename "$so")
  if [[ ! -f $stage/lib/$base ]]; then
    # Copy the file the SONAME points at, under the SONAME itself.
    # cp -a would copy a dangling symlink into lib/.
    cp -L "$so" "$stage/lib/$base"
    enqueue_deps "$stage/lib/$base"
  fi
done

cat > "$stage/mclauncher.sh" <<'EOF'
#!/bin/sh
here=$(CDPATH= cd -- "$(dirname "$0")" && pwd)
export LD_LIBRARY_PATH="$here/lib${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}"
export QT_PLUGIN_PATH="$here/plugins"
export QML_IMPORT_PATH="$here/qml"
export QT_QPA_PLATFORM="${QT_QPA_PLATFORM:-xcb}"
exec "$here/mclauncher" "$@"
EOF
chmod +x "$stage/mclauncher.sh" "$stage/mclauncher" "$stage/mclauncher-cli"
bash .github/scripts/pack-onefile.sh "$stage" dist/mclauncher-linux-amd64
