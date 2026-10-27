// Package java finds a javaw/java binary the way the 2014 launcher did:
// saved path, JAVA_HOME, then PATH. It does not download a JRE; the old
// updatemc.mcsky.net Java mirrors are gone.
package java

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func ExecutableNames() []string {
	if runtime.GOOS == "windows" {
		return []string{"javaw.exe", "java.exe"}
	}
	return []string{"java"}
}

// Find returns the first usable java binary. preferred is the user's saved path.
func Find(preferred string) (string, error) {
	if p := cleanExisting(preferred); p != "" {
		return p, nil
	}
	if home := os.Getenv("JAVA_HOME"); home != "" {
		for _, name := range ExecutableNames() {
			if p := cleanExisting(filepath.Join(home, "bin", name)); p != "" {
				return p, nil
			}
		}
	}
	for _, name := range ExecutableNames() {
		if p, err := exec.LookPath(name); err == nil {
			return p, nil
		}
	}
	return "", errNotFound
}

var errNotFound = errString("没有找到java路径,请手动指定")

type errString string

func (e errString) Error() string { return string(e) }

func cleanExisting(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return ""
	}
	if st, err := os.Stat(p); err == nil && !st.IsDir() {
		return p
	}
	return ""
}
