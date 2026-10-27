//go:build onefile

// Command onefile is the file a user downloads. The Qt libraries, QML plugins
// and skins are zipped inside it and unpacked into the user cache on first run.
package main

import (
	_ "embed"
	"os"
	"os/exec"
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
	cmd := exec.Command(exe, os.Args[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			os.Exit(ee.ExitCode())
		}
		boot.Fail(err)
	}
}
