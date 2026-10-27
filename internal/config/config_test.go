package config

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	cfg := Default(filepath.Join(dir, ".minecraft"))
	cfg.PlayerName = "Melody"
	cfg.MaxMemoryMB = 1536
	cfg.MemoryAuto = false
	cfg.Debug = true
	cfg.SelectedVersion = "1.12.2"
	cfg.IsolateGame = true
	cfg.FirstRun = false
	cfg.AccessToken = "token"
	if err := Save(cfg); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.PlayerName != "Melody" || got.MaxMemoryMB != 1536 || !got.Debug {
		t.Fatalf("round trip mismatch: %+v", got)
	}
	if got.AccessToken != "token" {
		t.Fatal("token not persisted")
	}
	info, err := osStat(filepath.Join(dir, "mclauncher", FileName))
	if err != nil {
		t.Fatal(err)
	}
	if info&0o077 != 0 {
		t.Fatalf("config should not be world/group readable, mode %#o", info)
	}
}
