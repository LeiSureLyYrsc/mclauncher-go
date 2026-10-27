package java

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"time"
)

// Runtime is one java binary found on disk.
type Runtime struct {
	Path    string
	Major   int
	Version string
}

var versionRe = regexp.MustCompile(`version "(?:1\.)?(\d+)[^"]*"`)

// ParseVersion reads the first version from `java -version` output.
// "1.8.0_392" is major 8. "17.0.9" is major 17.
func ParseVersion(out string) (major int, full string) {
	m := versionRe.FindStringSubmatch(out)
	if m == nil {
		return 0, ""
	}
	full = strings.TrimPrefix(m[0], `version "`)
	full = strings.TrimSuffix(full, `"`)
	if strings.HasPrefix(full, "1.") {
		rest := strings.TrimPrefix(full, "1.")
		return atoi(rest), full
	}
	return atoi(m[1]), full
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// Sniff looks through PATH, JAVA_HOME, common install directories, and
// <gameDir>/runtime. preferred is probed first when it exists.
func Sniff(gameDir, preferred string) []Runtime {
	seen := map[string]bool{}
	var out []Runtime
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		if seen[path] {
			return
		}
		seen[path] = true
		if rt, ok := probe(path); ok {
			out = append(out, rt)
		}
	}
	if p := cleanExisting(preferred); p != "" {
		add(p)
	}
	for _, name := range ExecutableNames() {
		if p, err := exec.LookPath(name); err == nil {
			add(p)
		}
	}
	if home := os.Getenv("JAVA_HOME"); home != "" {
		for _, name := range ExecutableNames() {
			add(filepath.Join(home, "bin", name))
		}
	}
	for _, root := range searchRoots(gameDir) {
		walkJava(root, 0, add)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Major != out[j].Major {
			return out[i].Major > out[j].Major
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// Choose picks a java binary. An explicit path is kept when it runs and,
// if wantMajor > 0, it is new enough. Otherwise the closest sniffed runtime
// that satisfies wantMajor is used.
func Choose(preferred, gameDir string, wantMajor int) (string, error) {
	list := Sniff(gameDir, preferred)
	if len(list) == 0 {
		return "", errNotFound
	}
	if p := cleanExisting(preferred); p != "" {
		for _, rt := range list {
			if sameFile(rt.Path, p) && (wantMajor <= 0 || rt.Major >= wantMajor) {
				return rt.Path, nil
			}
		}
	}
	var best *Runtime
	for i := range list {
		rt := &list[i]
		if wantMajor > 0 && rt.Major < wantMajor {
			continue
		}
		if best == nil || rt.Major < best.Major {
			best = rt
		}
	}
	if best == nil {
		return "", fmt.Errorf("需要 Java %d 或更新，只找到 %s", wantMajor, describe(list))
	}
	return best.Path, nil
}

func describe(list []Runtime) string {
	var b strings.Builder
	for i, rt := range list {
		if i > 0 {
			b.WriteString(", ")
		}
		fmt.Fprintf(&b, "Java %d", rt.Major)
	}
	if b.Len() == 0 {
		return "无"
	}
	return b.String()
}

func probe(path string) (Runtime, bool) {
	if cleanExisting(path) == "" {
		return Runtime{}, false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-version")
	out, err := cmd.CombinedOutput()
	if err != nil && len(out) == 0 {
		return Runtime{}, false
	}
	major, full := ParseVersion(string(out))
	if major == 0 {
		return Runtime{}, false
	}
	return Runtime{Path: path, Major: major, Version: full}, true
}

func sameFile(a, b string) bool {
	aa, err1 := filepath.Abs(a)
	bb, err2 := filepath.Abs(b)
	if err1 != nil || err2 != nil {
		return a == b
	}
	return aa == bb
}

func searchRoots(gameDir string) []string {
	var roots []string
	if gameDir != "" {
		roots = append(roots, filepath.Join(gameDir, "runtime"))
	}
	home, _ := os.UserHomeDir()
	switch runtime.GOOS {
	case "windows":
		for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "LocalAppData"} {
			if v := os.Getenv(env); v != "" {
				roots = append(roots,
					filepath.Join(v, "Java"),
					filepath.Join(v, "Eclipse Adoptium"),
					filepath.Join(v, "Microsoft"),
					filepath.Join(v, "BellSoft"),
					filepath.Join(v, "Zulu"),
					filepath.Join(v, "Amazon Corretto"),
					filepath.Join(v, "Programs", "Eclipse Adoptium"),
				)
			}
		}
	case "darwin":
		roots = append(roots,
			"/Library/Java/JavaVirtualMachines",
			filepath.Join(home, "Library/Java/JavaVirtualMachines"),
			"/opt/homebrew/opt",
			"/usr/local/opt",
		)
	default:
		roots = append(roots,
			"/usr/lib/jvm",
			"/usr/lib64/jvm",
			"/usr/java",
			"/opt/java",
			"/opt/jdk",
			filepath.Join(home, ".sdkman/candidates/java"),
		)
	}
	return roots
}

func walkJava(root string, depth int, add func(string)) {
	if depth > 5 {
		return
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		path := filepath.Join(root, name)
		if !e.IsDir() {
			if isJavaBinary(name) && depth <= 2 {
				add(path)
			}
			continue
		}
		if name == "bin" {
			for _, bin := range ExecutableNames() {
				add(filepath.Join(path, bin))
			}
			continue
		}
		if name == "." || name == ".." || strings.HasPrefix(name, ".") {
			continue
		}
		walkJava(path, depth+1, add)
	}
}

func isJavaBinary(name string) bool {
	for _, bin := range ExecutableNames() {
		if strings.EqualFold(name, bin) {
			return true
		}
	}
	return false
}
