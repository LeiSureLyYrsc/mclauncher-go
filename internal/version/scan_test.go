package version

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportAndScan(t *testing.T) {
	game := t.TempDir()
	bin := filepath.Join(game, "bin")
	if err := os.MkdirAll(bin, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "minecraft.jar"), []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !HasLegacyBin(game) {
		t.Fatal("expected legacy bin")
	}
	inst, err := ImportLegacy(game, "1.5.2-import")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(inst.Jar); err != nil {
		t.Fatal(err)
	}
	list, err := Scan(game)
	if err != nil || len(list) != 1 || list[0].ID != "1.5.2-import" {
		t.Fatalf("%v %v", list, err)
	}
	if err := Rename(game, "1.5.2-import", "old"); err != nil {
		t.Fatal(err)
	}
	list, _ = Scan(game)
	if len(list) != 1 || list[0].ID != "old" {
		t.Fatal(list)
	}
	if err := Remove(game, "old"); err != nil {
		t.Fatal(err)
	}
	list, _ = Scan(game)
	if len(list) != 0 {
		t.Fatal(list)
	}
}
