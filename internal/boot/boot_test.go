package boot

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractAndReuse(t *testing.T) {
	payload := zipBytes(t, map[string]string{
		"mclauncher":         "#!/bin/sh\necho hi\n",
		"lib/libQt6Core":     "qt",
		"qml/QtQuick/qmldir": "module QtQuick\n",
	})
	root := t.TempDir()
	dir, err := Ensure(root, payload)
	if err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dir, "lib", "libQt6Core"))
	if err != nil || string(got) != "qt" {
		t.Fatalf("lib = %q err=%v", got, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "qml", "QtQuick", "qmldir")); err != nil {
		t.Fatal(err)
	}
	again, err := Ensure(root, payload)
	if err != nil || again != dir {
		t.Fatalf("reuse %q err=%v", again, err)
	}
	exe := InnerExe(dir)
	if filepath.Base(exe) != innerBase() {
		t.Fatal(exe)
	}
}

func TestExtractRejectsEscape(t *testing.T) {
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	w, err := z.Create("../nope")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	if err := Extract(t.TempDir(), buf.Bytes()); err == nil {
		t.Fatal("expected reject")
	}
}

func TestLaunchEnv(t *testing.T) {
	dir := filepath.Join("/opt", "rt")
	env := LaunchEnv(dir, []string{"HOME=/home/a", "LD_LIBRARY_PATH=/old", "PATH=/usr/bin"})
	joined := strings.Join(env, "\n")
	if !strings.Contains(joined, "QT_PLUGIN_PATH="+filepath.Join(dir, "plugins")) {
		t.Fatal(joined)
	}
	if !strings.Contains(joined, "QML_IMPORT_PATH="+filepath.Join(dir, "qml")) {
		t.Fatal(joined)
	}
	if strings.Contains(joined, "LD_LIBRARY_PATH=/old\n") || strings.HasSuffix(joined, "LD_LIBRARY_PATH=/old") {
		t.Fatal(joined)
	}
}

func innerBase() string {
	if os.PathSeparator == '\\' {
		return "mclauncher.exe"
	}
	return "mclauncher"
}

func zipBytes(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for name, body := range files {
		w, err := z.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := z.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
