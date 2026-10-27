package ui

import (
	"os"
	"path/filepath"
)

func assetRoots() []string {
	var roots []string
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		roots = append(roots,
			filepath.Join(dir, "assets"),
			filepath.Join(dir, "..", "assets"),
		)
	}
	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots,
			filepath.Join(cwd, "assets"),
			filepath.Join(cwd, "..", "assets"),
			filepath.Join(cwd, "..", "..", "assets"),
		)
	}
	return roots
}

func findAsset(rel string) string {
	for _, root := range assetRoots() {
		p := filepath.Join(root, rel)
		st, err := os.Stat(p)
		if err != nil || st.IsDir() {
			continue
		}
		abs, err := filepath.Abs(p)
		if err == nil {
			return abs
		}
		return p
	}
	return rel
}

func findDir(rel string) string {
	for _, root := range assetRoots() {
		p := filepath.Join(root, rel)
		st, err := os.Stat(p)
		if err != nil || !st.IsDir() {
			continue
		}
		abs, err := filepath.Abs(p)
		if err == nil {
			return abs
		}
		return p
	}
	return rel
}

func skinFile(id string) string {
	switch id {
	case "sky":
		return "skins/sky.jpg"
	case "default", "desert":
		return "skins/default.png"
	default:
		return "skins/sunset.jpg"
	}
}
