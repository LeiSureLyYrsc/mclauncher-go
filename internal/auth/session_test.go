package auth

import "testing"

func TestOfflineUUIDStableAndVersion(t *testing.T) {
	a := OfflineUUID("Notch")
	b := OfflineUUID("Notch")
	if a != b || len(a) != 32 {
		t.Fatalf("uuid %q", a)
	}
	if a[12] != '3' {
		t.Fatalf("version nibble, got %q", a)
	}
	s, err := Offline("Notch")
	if err != nil {
		t.Fatal(err)
	}
	if s.UserType != "legacy" || s.AccessToken != "0" || !s.Offline {
		t.Fatalf("%+v", s)
	}
	dashed := s.UUIDDashed()
	if len(dashed) != 36 || dashed[8] != '-' {
		t.Fatal(dashed)
	}
	if _, err := Offline("  "); err == nil {
		t.Fatal("empty name should fail")
	}
}
