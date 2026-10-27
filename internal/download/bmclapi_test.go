package download

import "testing"

func TestRewriteBMCLAPI(t *testing.T) {
	cases := map[string]string{
		"https://piston-meta.mojang.com/mc/game/version_manifest_v2.json":                                                   "https://bmclapi2.bangbang93.com/mc/game/version_manifest_v2.json",
		"https://piston-data.mojang.com/v1/objects/abc/client.jar":                                                          "https://bmclapi2.bangbang93.com/v1/objects/abc/client.jar",
		"https://libraries.minecraft.net/com/google/guava/guava/17.0/guava-17.0.jar":                                        "https://bmclapi2.bangbang93.com/maven/com/google/guava/guava/17.0/guava-17.0.jar",
		"https://resources.download.minecraft.net/ab/abcd":                                                                  "https://bmclapi2.bangbang93.com/assets/ab/abcd",
		"https://files.minecraftforge.net/maven/net/minecraftforge/forge/1.12.2-14.23.5.2859/forge-1.12.2-14.23.5.2859.jar": "https://bmclapi2.bangbang93.com/maven/net/minecraftforge/forge/1.12.2-14.23.5.2859/forge-1.12.2-14.23.5.2859.jar",
		"https://maven.fabricmc.net/net/fabricmc/fabric-loader/0.15.11/fabric-loader-0.15.11.jar":                           "https://bmclapi2.bangbang93.com/maven/net/fabricmc/fabric-loader/0.15.11/fabric-loader-0.15.11.jar",
		"http://127.0.0.1:9/client.jar":                                                                                     "http://127.0.0.1:9/client.jar",
		"https://bmclapi2.bangbang93.com/maven/already.jar":                                                                 "https://bmclapi2.bangbang93.com/maven/already.jar",
	}
	for in, want := range cases {
		if got := Rewrite(in); got != want {
			t.Errorf("Rewrite(%s)\n got %s\nwant %s", in, got, want)
		}
	}
}

func TestMavenPath(t *testing.T) {
	got := MavenPath("com.google.guava:guava:17.0")
	want := "com/google/guava/guava/17.0/guava-17.0.jar"
	if got != want {
		t.Fatal(got)
	}
	got = MavenPath("org.lwjgl:lwjgl:3.3.1:natives-linux@jar")
	want = "org/lwjgl/lwjgl/3.3.1/lwjgl-3.3.1-natives-linux.jar"
	if got != want {
		t.Fatal(got)
	}
}
