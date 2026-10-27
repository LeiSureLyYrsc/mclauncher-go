package launch

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/LeiSureLyYrsc/mclauncher-go/internal/auth"
)

func TestAllowedRules(t *testing.T) {
	osName := "linux"
	if runtime.GOOS == "windows" {
		osName = "windows"
	} else if runtime.GOOS == "darwin" {
		osName = "osx"
	}
	rules := []Rule{
		{Action: "allow", OS: &OSRule{Name: osName}},
		{Action: "disallow", OS: &OSRule{Name: "nope"}},
	}
	if !Allowed(rules, nil) {
		t.Fatal("current os should be allowed")
	}
	onlyOther := []Rule{{Action: "allow", OS: &OSRule{Name: "definitely-not"}}}
	if Allowed(onlyOther, nil) {
		t.Fatal("unmatched rules must exclude")
	}
	if !Allowed(nil, nil) {
		t.Fatal("no rules means include")
	}
	feat := []Rule{{Action: "allow", Features: map[string]bool{"is_demo_user": true}}}
	if Allowed(feat, nil) {
		t.Fatal("feature rule should not match")
	}
	if !Allowed(feat, map[string]bool{"is_demo_user": true}) {
		t.Fatal("feature rule should match")
	}
}

func TestMavenPath(t *testing.T) {
	got := mavenPath("com.mojang:patchy:1.1", "")
	want := "com/mojang/patchy/1.1/patchy-1.1.jar"
	if got != want {
		t.Fatalf("got %s", got)
	}
	got = mavenPath("org.lwjgl:lwjgl:3.3.1:natives-linux", "")
	if !strings.HasSuffix(got, "lwjgl-3.3.1-natives-linux.jar") {
		t.Fatal(got)
	}
}

func TestBuildModernAndLegacy(t *testing.T) {
	dir := t.TempDir()
	game := filepath.Join(dir, ".minecraft")
	ver := filepath.Join(game, "versions", "1.8.8")
	if err := os.MkdirAll(ver, 0o755); err != nil {
		t.Fatal(err)
	}
	javaBin := filepath.Join(dir, "java")
	if err := os.WriteFile(javaBin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	js := `{
	  "id": "1.8.8",
	  "type": "release",
	  "mainClass": "net.minecraft.client.main.Main",
	  "minecraftArguments": "--username ${auth_player_name} --version ${version_name} --gameDir ${game_directory} --assetsDir ${assets_root} --assetIndex ${assets_index_name} --uuid ${auth_uuid} --accessToken ${auth_access_token} --userType ${user_type}",
	  "assets": "1.8",
	  "libraries": [
	    {"name": "com.mojang:patchy:1.1", "downloads": {"artifact": {"path": "com/mojang/patchy/1.1/patchy-1.1.jar", "sha1": "a", "size": 1}}},
	    {"name": "org.lwjgl:lwjgl-platform:2.9.4-nightly-20150209", "natives": {"linux": "natives-linux", "windows": "natives-windows", "osx": "natives-osx"}, "downloads": {"classifiers": {"natives-linux": {"path": "org/lwjgl/lwjgl-platform/2.9.4-nightly-20150209/lwjgl-platform-2.9.4-nightly-20150209-natives-linux.jar"}}}, "rules": [{"action": "allow"}]}
	  ]
	}`
	jp := filepath.Join(ver, "1.8.8.json")
	if err := os.WriteFile(jp, []byte(js), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ver, "1.8.8.jar"), []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	sess, err := auth.Offline("Melody")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := Build(Request{
		VersionJSON: jp,
		GameDir:     game,
		JavaBin:     javaBin,
		MemoryMB:    1024,
		Session:     sess,
	})
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(plan.Args, "\n")
	for _, want := range []string{
		javaBin,
		"-Xmx1024M",
		"net.minecraft.client.main.Main",
		"--username",
		"Melody",
		"patchy-1.1.jar",
		"-Dfml.ignoreInvalidMinecraftCertificates=true",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q\n%s", want, joined)
		}
	}
	if runtime.GOOS == "linux" && strings.Contains(joined, "natives-linux.jar") {
		t.Error("native classifier jar must not be on the classpath")
	}
	if plan.Dir != game {
		t.Fatal(plan.Dir)
	}

	legacy := `{
	  "id": "1.5.2",
	  "mainClass": "net.minecraft.client.Minecraft"
	}`
	ver2 := filepath.Join(game, "versions", "1.5.2")
	if err := os.MkdirAll(ver2, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ver2, "1.5.2.json"), []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ver2, "1.5.2.jar"), []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	plan, err = Build(Request{
		VersionJSON: filepath.Join(ver2, "1.5.2.json"),
		GameDir:     game,
		JavaBin:     javaBin,
		MemoryMB:    800,
		Session:     sess,
		Isolate:     true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Dir != ver2 {
		t.Fatalf("isolate dir %s", plan.Dir)
	}
	if !strings.Contains(strings.Join(plan.Args, " "), "-Xincgc") {
		t.Fatal(plan.Args)
	}
	if plan.Args[len(plan.Args)-2] != "Melody" {
		t.Fatal(plan.Args[len(plan.Args)-2:])
	}

	name, body := Script(plan)
	if name == "" || !strings.Contains(body, "Melody") {
		t.Fatal(name, body)
	}
}

func TestExpandConditionalArg(t *testing.T) {
	raw := []byte(`[{"rules":[{"action":"allow","features":{"is_demo_user":true}}],"value":"--demo"},"--username","${auth_player_name}"]`)
	var msgs []json.RawMessage
	if err := json.Unmarshal(raw, &msgs); err != nil {
		t.Fatal(err)
	}
	got, err := expandList(msgs, map[string]string{"auth_player_name": "A"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, " ") != "--username A" {
		t.Fatalf("%#v", got)
	}
	got, err = expandList(msgs, map[string]string{"auth_player_name": "A"}, map[string]bool{"is_demo_user": true})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(got, " ") != "--demo --username A" {
		t.Fatalf("%#v", got)
	}
}
