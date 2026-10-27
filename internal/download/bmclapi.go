package download

import (
	"net/url"
	"strings"
)

const (
	// SourceOfficial downloads from Mojang. Nothing is rewritten.
	SourceOfficial = "official"
	// SourceBMCLAPI sends Mojang, Forge and Fabric URLs through bmclapi2.
	SourceBMCLAPI = "bmclapi"

	bmclHost = "bmclapi2.bangbang93.com"
)

// NormalizeSource accepts the two public sources. Anything else is official.
func NormalizeSource(source string) string {
	switch strings.ToLower(strings.TrimSpace(source)) {
	case SourceBMCLAPI, "bmcl", "bangbang93":
		return SourceBMCLAPI
	default:
		return SourceOfficial
	}
}

func OtherSource(source string) string {
	if NormalizeSource(source) == SourceBMCLAPI {
		return SourceOfficial
	}
	return SourceBMCLAPI
}

// ManifestURL is the version list for this source.
func ManifestURL(source string) string {
	if NormalizeSource(source) == SourceBMCLAPI {
		return "https://" + bmclHost + "/mc/game/version_manifest_v2.json"
	}
	return "https://piston-meta.mojang.com/mc/game/version_manifest_v2.json"
}

// Rewrite sends Mojang, Forge, Fabric and NeoForge download URLs through the
// selected mirror. Official leaves them alone. Already-mirrored URLs stay.
func Rewrite(source, raw string) string {
	if NormalizeSource(source) != SourceBMCLAPI {
		return raw
	}
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

// Official maps a BMCLAPI url back to the public Mojang host so a failed
// node redirect can be retried without the mirror.
func Official(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return raw
	}
	host := strings.ToLower(u.Hostname())
	if host != bmclHost && !strings.HasSuffix(host, "."+bmclHost) {
		return raw
	}
	path := u.EscapedPath()
	switch {
	case strings.HasPrefix(path, "/maven/"):
		return "https://libraries.minecraft.net" + strings.TrimPrefix(path, "/maven")
	case strings.HasPrefix(path, "/assets/"):
		return "https://resources.download.minecraft.net" + strings.TrimPrefix(path, "/assets")
	case strings.HasPrefix(path, "/v1/objects/"):
		return "https://piston-data.mojang.com" + path
	case strings.HasPrefix(path, "/mc/"), strings.HasPrefix(path, "/v1/"):
		return "https://piston-meta.mojang.com" + path
	default:
		return raw
	}
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
