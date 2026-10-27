package java

import "testing"

func TestParseVersion(t *testing.T) {
	major, full := ParseVersion("openjdk version \"1.8.0_392\"\nOpenJDK Runtime Environment")
	if major != 8 || full != "1.8.0_392" {
		t.Fatalf("8: %d %q", major, full)
	}
	major, full = ParseVersion("openjdk version \"21.0.2\" 2024-01-16")
	if major != 21 || full != "21.0.2" {
		t.Fatalf("21: %d %q", major, full)
	}
	major, _ = ParseVersion("java version \"17.0.9\"")
	if major != 17 {
		t.Fatal(major)
	}
}
