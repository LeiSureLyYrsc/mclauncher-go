// Package launch turns a Minecraft version JSON plus a login session into the
// javaw command line the 2014 launcher built by hand: classpath, natives,
// -Xmx, and ${auth_*} substitution. Both the pre-1.13 minecraftArguments
// string and the modern arguments.game / arguments.jvm arrays are supported.
package launch

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/LeiSureLyYrsc/mclauncher-go/internal/auth"
)

// Version is the subset of Mojang's version JSON the launcher needs.
type Version struct {
	ID                 string          `json:"id"`
	MainClass          string          `json:"mainClass"`
	MinecraftArguments string          `json:"minecraftArguments"`
	Arguments          *Arguments      `json:"arguments"`
	Libraries          []Library       `json:"libraries"`
	Assets             string          `json:"assets"`
	AssetIndex         *AssetIndexRef  `json:"assetIndex"`
	Downloads          json.RawMessage `json:"downloads"`
	Jar                string          `json:"jar"`
	InheritsFrom       string          `json:"inheritsFrom"`
	Type               string          `json:"type"`
}

type Arguments struct {
	Game []json.RawMessage `json:"game"`
	JVM  []json.RawMessage `json:"jvm"`
}

type Library struct {
	Name      string            `json:"name"`
	Downloads *LibDownloads     `json:"downloads"`
	Rules     []Rule            `json:"rules"`
	Natives   map[string]string `json:"natives"`
	Extract   *Extract          `json:"extract"`
	URL       string            `json:"url"`
}

type LibDownloads struct {
	Artifact    *Artifact           `json:"artifact"`
	Classifiers map[string]Artifact `json:"classifiers"`
}

type Artifact struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}

type Extract struct {
	Exclude []string `json:"exclude"`
}

type Rule struct {
	Action   string          `json:"action"`
	OS       *OSRule         `json:"os"`
	Features map[string]bool `json:"features"`
}

type OSRule struct {
	Name string `json:"name"`
	Arch string `json:"arch"`
}

type AssetIndexRef struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// Request is everything Build needs besides the version JSON.
type Request struct {
	VersionJSON string
	GameDir     string // .minecraft root
	JavaBin     string
	MemoryMB    int
	Session     auth.Session
	Isolate     bool
	Features    map[string]bool
	ExtraJVM    []string
	// AuthlibInjector is an optional -javaagent jar for third-party Yggdrasil.
	AuthlibInjector string
	AuthServer      string
}

// Plan is a ready-to-exec command. Args[0] is the java binary.
type Plan struct {
	Args       []string
	Dir        string
	NativesDir string
	LogHint    string
	VersionID  string
}

type osInfo struct {
	name string // minecraft rule name: windows, osx, linux
	arch string // x86, x86_64
	bits string // 32 or 64, for ${arch} in old natives
}

func currentOS() osInfo {
	name := runtime.GOOS
	switch name {
	case "darwin":
		name = "osx"
	case "windows", "linux":
	default:
		// treat other unix as linux for library rules
		if name != "windows" && name != "osx" {
			name = "linux"
		}
	}
	arch := "x86"
	bits := "32"
	switch runtime.GOARCH {
	case "amd64", "arm64":
		arch = "x86_64"
		bits = "64"
	}
	return osInfo{name: name, arch: arch, bits: bits}
}

func (o osInfo) ruleName() string { return o.name }

// Allowed reports whether a library or argument rule list includes this OS.
// No rules means allowed. Otherwise the last matching rule wins; if none
// match, the item is excluded. Feature-gated rules match only when the
// feature map says so.
func Allowed(rules []Rule, features map[string]bool) bool {
	if len(rules) == 0 {
		return true
	}
	osys := currentOS()
	matched := false
	allow := false
	for _, r := range rules {
		if !ruleMatches(r, osys, features) {
			continue
		}
		matched = true
		allow = r.Action == "allow"
	}
	return matched && allow
}

func ruleMatches(r Rule, osys osInfo, features map[string]bool) bool {
	if r.OS != nil {
		if r.OS.Name != "" && r.OS.Name != osys.ruleName() {
			return false
		}
		if r.OS.Arch != "" && r.OS.Arch != osys.arch {
			return false
		}
	}
	for k, want := range r.Features {
		got := features[k]
		if got != want {
			return false
		}
	}
	return true
}

// LoadVersion reads a <id>.json file.
func LoadVersion(path string) (Version, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Version{}, err
	}
	var v Version
	if err := json.Unmarshal(b, &v); err != nil {
		return Version{}, fmt.Errorf("版本 JSON 无法解析: %w", err)
	}
	if v.ID == "" {
		v.ID = strings.TrimSuffix(filepath.Base(path), ".json")
	}
	return v, nil
}

// Build constructs the java command. It also extracts natives when present.
func Build(req Request) (Plan, error) {
	if req.JavaBin == "" {
		return Plan{}, fmt.Errorf("没有选择java路径")
	}
	if req.GameDir == "" {
		return Plan{}, fmt.Errorf("没有游戏目录")
	}
	if strings.TrimSpace(req.Session.Name) == "" {
		return Plan{}, fmt.Errorf("没有填写游戏名字")
	}
	v, err := LoadVersion(req.VersionJSON)
	if err != nil {
		return Plan{}, err
	}
	if v.MainClass == "" {
		return Plan{}, fmt.Errorf("版本 %s 没有 mainClass", v.ID)
	}
	mem := req.MemoryMB
	if mem < 256 {
		mem = 1024
	}
	verDir := filepath.Dir(req.VersionJSON)
	natives := filepath.Join(verDir, "natives")
	if err := os.MkdirAll(natives, 0o755); err != nil {
		return Plan{}, err
	}
	cp, err := classpath(v, req.GameDir, natives)
	if err != nil {
		return Plan{}, err
	}
	if isLegacyMain(v.MainClass) {
		for _, extra := range []string{"lwjgl.jar", "lwjgl_util.jar", "jinput.jar"} {
			p := filepath.Join(verDir, extra)
			if _, err := os.Stat(p); err == nil {
				cp = append(cp, p)
			}
		}
	}
	client := clientJar(v, verDir)
	if _, err := os.Stat(client); err != nil {
		return Plan{}, fmt.Errorf("没有找到客户端 %s，请先在下载中心安装", filepath.Base(client))
	}
	cp = append(cp, client)

	gameDir := req.GameDir
	if req.Isolate {
		gameDir = verDir
	}
	assetsRoot := filepath.Join(req.GameDir, "assets")
	indexID := v.Assets
	if v.AssetIndex != nil && v.AssetIndex.ID != "" {
		indexID = v.AssetIndex.ID
	}
	if indexID == "" {
		indexID = "legacy"
	}
	vars := map[string]string{
		"auth_player_name":    req.Session.Name,
		"auth_uuid":           strings.ReplaceAll(req.Session.UUID, "-", ""),
		"auth_access_token":   or(req.Session.AccessToken, "0"),
		"auth_session":        sessionToken(req.Session),
		"user_type":           or(req.Session.UserType, "legacy"),
		"user_properties":     req.Session.PropertiesJSON(),
		"version_name":        v.ID,
		"version_type":        or(v.Type, "release"),
		"game_directory":      gameDir,
		"assets_root":         assetsRoot,
		"game_assets":         filepath.Join(assetsRoot, "virtual", "legacy"),
		"assets_index_name":   indexID,
		"natives_directory":   natives,
		"launcher_name":       "mclauncher-go",
		"launcher_version":    "1.0.0",
		"classpath":           joinCP(cp),
		"classpath_separator": string(os.PathListSeparator),
		"library_directory":   filepath.Join(req.GameDir, "libraries"),
	}

	jvm := []string{req.JavaBin}
	if req.AuthlibInjector != "" {
		agent := req.AuthlibInjector
		if req.AuthServer != "" {
			agent += "=" + req.AuthServer
		}
		jvm = append(jvm, "-javaagent:"+agent)
	}
	jvm = append(jvm,
		fmt.Sprintf("-Xmx%dM", mem),
		"-Djava.library.path="+natives,
		"-Dminecraft.launcher.brand=mclauncher-go",
		"-Dminecraft.launcher.version=1.0.0",
		"-Dfml.ignoreInvalidMinecraftCertificates=true",
		"-Dfml.ignorePatchDiscrepancies=true",
	)
	jvm = append(jvm, req.ExtraJVM...)
	if v.Arguments != nil && len(v.Arguments.JVM) > 0 {
		extra, err := expandList(v.Arguments.JVM, vars, req.Features)
		if err != nil {
			return Plan{}, err
		}
		// Drop a second -Xmx if the json also specifies one; ours stays first.
		jvm = append(jvm, filterDupXmx(extra)...)
	} else if isLegacyMain(v.MainClass) {
		jvm = append(jvm, "-Xincgc")
	}
	jvm = append(jvm, "-cp", joinCP(cp), v.MainClass)

	var game []string
	if v.Arguments != nil && len(v.Arguments.Game) > 0 {
		game, err = expandList(v.Arguments.Game, vars, req.Features)
		if err != nil {
			return Plan{}, err
		}
	} else if v.MinecraftArguments != "" {
		game = splitArgs(substitute(v.MinecraftArguments, vars))
	} else if isLegacyMain(v.MainClass) {
		game = []string{req.Session.Name, sessionToken(req.Session)}
	} else {
		return Plan{}, fmt.Errorf("版本 %s 没有启动参数", v.ID)
	}

	return Plan{
		Args:       append(jvm, game...),
		Dir:        gameDir,
		NativesDir: natives,
		VersionID:  v.ID,
		LogHint:    "输出日志.txt",
	}, nil
}

func isLegacyMain(main string) bool {
	return main == "net.minecraft.client.Minecraft"
}

func sessionToken(s auth.Session) string {
	token := or(s.AccessToken, "0")
	id := strings.ReplaceAll(s.UUID, "-", "")
	if id == "" {
		return token
	}
	return token + ":" + id
}

func or(a, b string) string {
	if a == "" {
		return b
	}
	return a
}

func clientJar(v Version, verDir string) string {
	name := v.ID
	if v.Jar != "" {
		name = v.Jar
	}
	return filepath.Join(verDir, name+".jar")
}

func classpath(v Version, gameDir, natives string) ([]string, error) {
	var cp []string
	for _, lib := range v.Libraries {
		if !Allowed(lib.Rules, nil) {
			continue
		}
		art := libraryArtifact(lib)
		if art == nil || art.Path == "" && lib.Name == "" {
			// natives-only library
		} else {
			path := artifactPath(*art, lib)
			if path != "" && !isNativeOnly(lib, path) {
				cp = append(cp, filepath.Join(gameDir, "libraries", filepath.FromSlash(path)))
			}
		}
		if err := extractNatives(lib, gameDir, natives); err != nil {
			return nil, err
		}
	}
	return cp, nil
}

func isNativeOnly(lib Library, path string) bool {
	if len(lib.Natives) > 0 && lib.Downloads != nil && lib.Downloads.Artifact == nil {
		return true
	}
	base := filepath.Base(path)
	return strings.Contains(base, "natives-")
}

func libraryArtifact(lib Library) *Artifact {
	if lib.Downloads != nil && lib.Downloads.Artifact != nil {
		return lib.Downloads.Artifact
	}
	if lib.Name == "" {
		return nil
	}
	path := mavenPath(lib.Name, "")
	return &Artifact{Path: path}
}

func artifactPath(a Artifact, lib Library) string {
	if a.Path != "" {
		return a.Path
	}
	return mavenPath(lib.Name, "")
}

func mavenPath(name, classifier string) string {
	// group:artifact:version[:classifier][@ext]
	ext := "jar"
	if i := strings.LastIndex(name, "@"); i >= 0 {
		ext = name[i+1:]
		name = name[:i]
	}
	parts := strings.Split(name, ":")
	if len(parts) < 3 {
		return ""
	}
	group := strings.ReplaceAll(parts[0], ".", "/")
	artifact, version := parts[1], parts[2]
	if classifier == "" && len(parts) >= 4 {
		classifier = parts[3]
	}
	file := artifact + "-" + version
	if classifier != "" {
		file += "-" + classifier
	}
	file += "." + ext
	return group + "/" + artifact + "/" + version + "/" + file
}

func extractNatives(lib Library, gameDir, natives string) error {
	if lib.Downloads == nil {
		return nil
	}
	key := nativeClassifier(lib)
	if key == "" {
		return nil
	}
	art, ok := lib.Downloads.Classifiers[key]
	if !ok {
		return nil
	}
	path := art.Path
	if path == "" {
		path = mavenPath(lib.Name, key)
	}
	jar := filepath.Join(gameDir, "libraries", filepath.FromSlash(path))
	if _, err := os.Stat(jar); err != nil {
		return nil // installer fills it later; launch will fail clearly if java needs it
	}
	return unzipNatives(jar, natives, lib.Extract)
}

func nativeClassifier(lib Library) string {
	if len(lib.Natives) == 0 {
		return ""
	}
	osys := currentOS()
	tmpl, ok := lib.Natives[osys.ruleName()]
	if !ok {
		return ""
	}
	return strings.ReplaceAll(tmpl, "${arch}", osys.bits)
}

func unzipNatives(jar, dest string, ex *Extract) error {
	r, err := zip.OpenReader(jar)
	if err != nil {
		return err
	}
	defer r.Close()
	if err := os.MkdirAll(dest, 0o755); err != nil {
		return err
	}
	for _, f := range r.File {
		name := filepath.Clean(f.Name)
		if name == "." || strings.HasPrefix(name, "..") || strings.Contains(name, ".."+string(os.PathSeparator)) {
			continue
		}
		if excluded(name, ex) {
			continue
		}
		if f.FileInfo().IsDir() {
			continue
		}
		// only ship native libraries, skip class files
		low := strings.ToLower(name)
		if strings.HasSuffix(low, ".class") || strings.HasSuffix(low, ".txt") || strings.HasSuffix(low, ".git") {
			continue
		}
		target := filepath.Join(dest, filepath.Base(name))
		if err := extractFile(f, target); err != nil {
			return err
		}
	}
	return nil
}

func excluded(name string, ex *Extract) bool {
	norm := strings.ReplaceAll(name, "\\", "/")
	if strings.HasPrefix(norm, "META-INF/") {
		return true
	}
	if ex == nil {
		return false
	}
	for _, p := range ex.Exclude {
		p = strings.ReplaceAll(p, "\\", "/")
		if strings.HasPrefix(norm, p) || strings.Contains(norm, p) {
			return true
		}
	}
	return false
}

func extractFile(f *zip.File, target string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, rc)
	return err
}

type argObj struct {
	Rules []Rule          `json:"rules"`
	Value json.RawMessage `json:"value"`
}

func expandList(raw []json.RawMessage, vars map[string]string, features map[string]bool) ([]string, error) {
	var out []string
	for _, item := range raw {
		item = bytesTrim(item)
		if len(item) == 0 {
			continue
		}
		if item[0] == '"' {
			var s string
			if err := json.Unmarshal(item, &s); err != nil {
				return nil, err
			}
			out = append(out, substitute(s, vars))
			continue
		}
		var obj argObj
		if err := json.Unmarshal(item, &obj); err != nil {
			return nil, err
		}
		if !Allowed(obj.Rules, features) {
			continue
		}
		val := bytesTrim(obj.Value)
		if len(val) == 0 {
			continue
		}
		if val[0] == '"' {
			var s string
			if err := json.Unmarshal(val, &s); err != nil {
				return nil, err
			}
			out = append(out, substitute(s, vars))
			continue
		}
		var arr []string
		if err := json.Unmarshal(val, &arr); err != nil {
			return nil, err
		}
		for _, s := range arr {
			out = append(out, substitute(s, vars))
		}
	}
	return out, nil
}

func bytesTrim(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

func substitute(s string, vars map[string]string) string {
	return os.Expand(s, func(k string) string {
		if v, ok := vars[k]; ok {
			return v
		}
		return "${" + k + "}"
	})
}

func splitArgs(s string) []string {
	return strings.Fields(s)
}

func joinCP(cp []string) string {
	return strings.Join(cp, string(os.PathListSeparator))
}

func filterDupXmx(args []string) []string {
	var out []string
	for _, a := range args {
		if strings.HasPrefix(a, "-Xmx") || strings.HasPrefix(a, "-Xms") {
			continue
		}
		out = append(out, a)
	}
	return out
}

// Script writes the debug launcher script the original called mclauncher.bat.
func Script(plan Plan) (string, string) {
	if runtime.GOOS == "windows" {
		var b strings.Builder
		b.WriteString("@echo off\r\n")
		fmt.Fprintf(&b, "cd /d %q\r\n", plan.Dir)
		b.WriteString(quoteWin(plan.Args))
		b.WriteString(" >输出日志.txt 2>&1\r\npause\r\n")
		return "mclauncher.bat", b.String()
	}
	var b strings.Builder
	b.WriteString("#!/bin/sh\n")
	fmt.Fprintf(&b, "cd %q || exit 1\n", plan.Dir)
	b.WriteString("exec ")
	for i, a := range plan.Args {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(shellQuote(a))
	}
	b.WriteString(" >输出日志.txt 2>&1\n")
	return "mclauncher.sh", b.String()
}

func shellQuote(s string) string {
	if s == "" {
		return "''"
	}
	if !strings.ContainsAny(s, " \t\n'\"\\$`") {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func quoteWin(args []string) string {
	var b strings.Builder
	for i, a := range args {
		if i > 0 {
			b.WriteByte(' ')
		}
		if strings.ContainsAny(a, " \t\"") {
			b.WriteByte('"')
			b.WriteString(strings.ReplaceAll(a, `"`, `\"`))
			b.WriteByte('"')
		} else {
			b.WriteString(a)
		}
	}
	return b.String()
}
