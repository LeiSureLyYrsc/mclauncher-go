//go:build onefile

// Command onefile is the file a user downloads. The Qt libraries, QML plugins
// and skins are zipped inside it and unpacked into the user cache on first run.
package main

import (
	_ "embed"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"

	"github.com/LeiSureLyYrsc/mclauncher-go/internal/boot"
)

//go:embed payload.zip
var payload []byte

func main() {
	cache, err := os.UserCacheDir()
	if err != nil {
		cache = os.TempDir()
	}
	dir, err := boot.Ensure(cache, payload)
	if err != nil {
		boot.Fail(err)
	}
	exe := boot.InnerExe(dir)
	if _, err := os.Stat(exe); err != nil {
		boot.Fail(err)
	}
	argv := append([]string{exe}, os.Args[1:]...)
	env := boot.LaunchEnv(dir, os.Environ())
	if runtime.GOOS != "windows" {
		if err := syscall.Exec(exe, argv, env); err != nil {
			boot.Fail(err)
		}
		return
	}
	if err := os.MkdirAll(filepath.Dir(boot.LogFile()), 0o755); err != nil {
		boot.Fail(err)
	}
	logf, err := os.OpenFile(boot.LogFile(), os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		boot.Fail(err)
	}
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, logf)
	cmd.Stderr = io.MultiWriter(os.Stderr, logf)
	runErr := cmd.Run()
	logf.Close()
	if runErr != nil {
		msg := runErr.Error()
		if b, err := os.ReadFile(boot.LogFile()); err == nil && len(b) > 0 {
			msg = string(b)
			if len(msg) > 1200 {
				msg = msg[len(msg)-1200:]
			}
		}
		boot.Fail(errors.New(msg))
	}
}
