// Package version lists installed games and imports the pre-1.6 bin/ layout
// the original launcher called “导入旧版本”.
package version

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Installed struct {
	ID   string
	Dir  string
	JSON string
	Jar  string
	Type string
}

// Scan reads .minecraft/versions/*/*.json.
func Scan(gameDir string) ([]Installed, error) {
	root := filepath.Join(gameDir, "versions")
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Installed
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		id := e.Name()
		dir := filepath.Join(root, id)
		js := filepath.Join(dir, id+".json")
		jar := filepath.Join(dir, id+".jar")
		if _, err := os.Stat(js); err != nil {
			continue
		}
		typ := ""
		if b, err := os.ReadFile(js); err == nil {
			var meta struct {
				Type string `json:"type"`
			}
			_ = json.Unmarshal(b, &meta)
			typ = meta.Type
		}
		out = append(out, Installed{ID: id, Dir: dir, JSON: js, Jar: jar, Type: typ})
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i].ID) < strings.ToLower(out[j].ID) })
	return out, nil
}

// HasLegacyBin reports the .minecraft/bin/minecraft.jar layout from before 1.6.
func HasLegacyBin(gameDir string) bool {
	_, err := os.Stat(filepath.Join(gameDir, "bin", "minecraft.jar"))
	return err == nil
}

// ImportLegacy copies bin/minecraft.jar into versions/<name> and writes a
// minimal JSON so the new launcher can start it. It does not delete the bin.
func ImportLegacy(gameDir, name string) (Installed, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Installed{}, fmt.Errorf("输入版本名称")
	}
	if strings.ContainsAny(name, `/\`) {
		return Installed{}, fmt.Errorf("版本名称不能包含路径分隔符")
	}
	src := filepath.Join(gameDir, "bin", "minecraft.jar")
	if _, err := os.Stat(src); err != nil {
		return Installed{}, fmt.Errorf("没有找到旧版 bin/minecraft.jar")
	}
	dir := filepath.Join(gameDir, "versions", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Installed{}, err
	}
	dst := filepath.Join(dir, name+".jar")
	if err := copyFile(src, dst); err != nil {
		return Installed{}, err
	}
	// keep sibling jars (lwjgl, jinput) beside the version for the classpath
	bin := filepath.Join(gameDir, "bin")
	for _, extra := range []string{"lwjgl.jar", "lwjgl_util.jar", "jinput.jar"} {
		from := filepath.Join(bin, extra)
		if _, err := os.Stat(from); err == nil {
			_ = copyFile(from, filepath.Join(dir, extra))
		}
	}
	natives := filepath.Join(bin, "natives")
	if st, err := os.Stat(natives); err == nil && st.IsDir() {
		_ = copyDir(natives, filepath.Join(dir, "natives"))
	}
	meta := map[string]any{
		"id":                 name,
		"type":               "release",
		"mainClass":          "net.minecraft.client.Minecraft",
		"assets":             "legacy",
		"minecraftArguments": "${auth_player_name} ${auth_session}",
	}
	b, _ := json.MarshalIndent(meta, "", "  ")
	js := filepath.Join(dir, name+".json")
	if err := os.WriteFile(js, append(b, '\n'), 0o644); err != nil {
		return Installed{}, err
	}
	marker := filepath.Join(gameDir, "bin", "已经导入.删除本文件重新导入")
	_ = os.WriteFile(marker, []byte("imported by mclauncher-go\n"), 0o644)
	return Installed{ID: name, Dir: dir, JSON: js, Jar: dst, Type: "release"}, nil
}

func Remove(gameDir, id string) error {
	if id == "" || strings.ContainsAny(id, `/\`) {
		return fmt.Errorf("无效的版本")
	}
	return os.RemoveAll(filepath.Join(gameDir, "versions", id))
}

func Rename(gameDir, id, next string) error {
	next = strings.TrimSpace(next)
	if id == "" || next == "" || strings.ContainsAny(id+next, `/\`) {
		return fmt.Errorf("无效的版本名称")
	}
	from := filepath.Join(gameDir, "versions", id)
	to := filepath.Join(gameDir, "versions", next)
	if _, err := os.Stat(to); err == nil {
		return fmt.Errorf("版本 %s 已存在", next)
	}
	if err := os.Rename(from, to); err != nil {
		return err
	}
	_ = os.Rename(filepath.Join(to, id+".json"), filepath.Join(to, next+".json"))
	_ = os.Rename(filepath.Join(to, id+".jar"), filepath.Join(to, next+".jar"))
	rewriteID(filepath.Join(to, next+".json"), next)
	return nil
}

func rewriteID(path, id string) {
	b, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var m map[string]any
	if json.Unmarshal(b, &m) != nil {
		return
	}
	m["id"] = id
	out, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, append(out, '\n'), 0o644)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}
