// Package boot unpacks the Qt runtime that was embedded in the single-file
// launcher and builds the environment the real window needs.
package boot

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// RuntimeDir is where this exact payload is unpacked. A new build hashes to
// a new directory, so an old runtime is never mixed with a new one.
func RuntimeDir(cacheRoot string, payload []byte) string {
	sum := sha256.Sum256(payload)
	id := hex.EncodeToString(sum[:8])
	return filepath.Join(cacheRoot, "mclauncher", "runtime", id)
}

// Ensure unpacks payload under cacheRoot and returns the directory that
// contains the real launcher. A second launch with the same bytes does no work.
func Ensure(cacheRoot string, payload []byte) (string, error) {
	dir := RuntimeDir(cacheRoot, payload)
	ready := filepath.Join(dir, ".ready")
	if _, err := os.Stat(ready); err == nil {
		return dir, nil
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return "", err
	}
	lockPath := dir + ".lock"
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o644)
	if err != nil {
		deadline := time.Now().Add(2 * time.Minute)
		for time.Now().Before(deadline) {
			if _, err := os.Stat(ready); err == nil {
				return dir, nil
			}
			time.Sleep(200 * time.Millisecond)
		}
		return "", fmt.Errorf("运行库正在被另一个启动器展开")
	}
	defer lock.Close()
	defer os.Remove(lockPath)

	tmp := dir + ".tmp"
	os.RemoveAll(tmp)
	if err := Extract(tmp, payload); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	if err := os.WriteFile(filepath.Join(tmp, ".ready"), []byte("ok\n"), 0o644); err != nil {
		os.RemoveAll(tmp)
		return "", err
	}
	os.RemoveAll(dir)
	if err := os.Rename(tmp, dir); err != nil {
		return "", err
	}
	sweepOld(filepath.Dir(dir), filepath.Base(dir))
	return dir, nil
}

// Extract writes a zip archive into dir. Names that escape dir are rejected.
func Extract(dir string, payload []byte) error {
	r, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, f := range r.File {
		if err := extractOne(dir, f); err != nil {
			return err
		}
	}
	for _, name := range []string{"mclauncher", "mclauncher.exe", "mclauncher.sh", "mclauncher-cli", "mclauncher-cli.exe"} {
		p := filepath.Join(dir, name)
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		if err := os.Chmod(p, st.Mode().Perm()|0o755); err != nil {
			return err
		}
	}
	return nil
}

func extractOne(dir string, f *zip.File) error {
	name := filepath.Clean(filepath.FromSlash(f.Name))
	if name == "." || name == "" {
		return nil
	}
	if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
		return fmt.Errorf("压缩包路径非法: %s", f.Name)
	}
	dest := filepath.Join(dir, name)
	if !strings.HasPrefix(dest, dir+string(os.PathSeparator)) && dest != dir {
		return fmt.Errorf("压缩包路径非法: %s", f.Name)
	}
	mode := f.Mode()
	if mode&os.ModeSymlink != 0 {
		return fmt.Errorf("压缩包不含符号链接: %s", f.Name)
	}
	if f.FileInfo().IsDir() {
		return os.MkdirAll(dest, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	perm := mode.Perm()
	if perm == 0 {
		perm = 0o644
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, rc)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// InnerExe is the Qt window inside an unpacked runtime.
func InnerExe(dir string) string {
	name := "mclauncher"
	if runtime.GOOS == "windows" {
		name = "mclauncher.exe"
	}
	return filepath.Join(dir, name)
}

// LaunchEnv points Qt at the plugins and libraries that were unpacked next
// to the real window. The process working directory is left alone.
func LaunchEnv(dir string, base []string) []string {
	plugin := pluginRoot(dir)
	qml := ""
	if st, err := os.Stat(filepath.Join(dir, "qml")); err == nil && st.IsDir() {
		qml = filepath.Join(dir, "qml")
	}
	lib := filepath.Join(dir, "lib")
	drop := map[string]bool{
		"QT_PLUGIN_PATH": true, "QML_IMPORT_PATH": true, "QML2_IMPORT_PATH": true,
		"LD_LIBRARY_PATH": true,
	}
	if runtime.GOOS == "windows" {
		drop["PATH"] = true
	}
	out := make([]string, 0, len(base)+5)
	var oldPath, oldLib string
	for _, e := range base {
		key, val, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		if drop[key] {
			switch key {
			case "PATH":
				oldPath = val
			case "LD_LIBRARY_PATH":
				oldLib = val
			}
			continue
		}
		out = append(out, e)
	}
	if plugin != "" {
		out = append(out, "QT_PLUGIN_PATH="+plugin)
	}
	if qml != "" {
		out = append(out, "QML_IMPORT_PATH="+qml, "QML2_IMPORT_PATH="+qml)
	}
	if runtime.GOOS == "linux" {
		if oldLib != "" {
			lib = lib + string(os.PathListSeparator) + oldLib
		}
		out = append(out, "LD_LIBRARY_PATH="+lib)
	}
	if runtime.GOOS == "windows" {
		path := dir
		if oldPath != "" {
			path = path + string(os.PathListSeparator) + oldPath
		}
		out = append(out, "PATH="+path)
	}
	return out
}

// pluginRoot is the directory Qt searches for platforms/. Linux stages that
// under plugins/; windeployqt puts it next to the exe.
func pluginRoot(dir string) string {
	if st, err := os.Stat(filepath.Join(dir, "plugins", "platforms")); err == nil && st.IsDir() {
		return filepath.Join(dir, "plugins")
	}
	if st, err := os.Stat(filepath.Join(dir, "platforms")); err == nil && st.IsDir() {
		return dir
	}
	return ""
}

func LogFile() string {
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	return filepath.Join(cache, "mclauncher", "boot.log")
}

func sweepOld(parent, keep string) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() || e.Name() == keep {
			continue
		}
		os.RemoveAll(filepath.Join(parent, e.Name()))
	}
}
