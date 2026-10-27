package download

import (
	"net/url"
	"strings"
)

const (
	// ManifestURL is BMCLAPI's copy of Mojang's version_manifest_v2.
	ManifestURL = "https://bmclapi2.bangbang93.com/mc/game/version_manifest_v2.json"
	bmclHost    = "bmclapi2.bangbang93.com"
)

// Rewrite sends Mojang, Forge, Fabric and NeoForge download URLs through BMCLAPI.
// Anything else, including tests and already-mirrored URLs, is left alone.
func Rewrite(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return raw
	}
	host := strings.ToLower(u.Hostname())
	if host == bmclHost || strings.HasSuffix(host, "."+bmclHost) {
		return raw
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	switch host {
	case "piston-meta.mojang.com", "piston-data.mojang.com", "launchermeta.mojang.com", "launcher.mojang.com":
		return mirror(path)
	case "libraries.minecraft.net":
		return mirror("/maven" + path)
	case "resources.download.minecraft.net":
		return mirror("/assets" + path)
	case "files.minecraftforge.net":
		if rest, ok := strings.CutPrefix(path, "/maven"); ok {
			return mirror("/maven" + rest)
		}
	case "maven.minecraftforge.net", "maven.fabricmc.net":
		return mirror("/maven" + path)
	case "meta.fabricmc.net":
		return mirror("/fabric-meta" + path)
	case "maven.neoforged.net":
		return mirror("/maven" + strings.TrimPrefix(path, "/releases"))
	}
	return raw
}

func mirror(path string) string {
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return "https://" + bmclHost + path
}

// MavenPath turns a library coordinate into a maven relative path.
// "com.google.guava:guava:17.0" -> "com/google/guava/guava/17.0/guava-17.0.jar"
func MavenPath(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	ext := "jar"
	if i := strings.LastIndex(name, "@"); i >= 0 {
		ext = name[i+1:]
		name = name[:i]
	}
	parts := strings.Split(name, ":")
	if len(parts) < 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return ""
	}
	file := parts[1] + "-" + parts[2]
	if len(parts) >= 4 && parts[3] != "" {
		file += "-" + parts[3]
	}
	file += "." + ext
	return strings.ReplaceAll(parts[0], ".", "/") + "/" + parts[1] + "/" + parts[2] + "/" + file
}
