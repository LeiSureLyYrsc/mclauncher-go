package download

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkipsGoodSHA(t *testing.T) {
	payload := []byte("client-bytes")
	sum := sha1.Sum(payload)
	sha := hex.EncodeToString(sum[:])
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch {
		case strings.HasSuffix(r.URL.Path, ".json") && !strings.Contains(r.URL.Path, "indexes"):
			w.Write([]byte(`{
				"id":"1.0-test",
				"downloads":{"client":{"url":"` + srvURL(r, "/client.jar") + `","sha1":"` + sha + `","size":1}},
				"libraries":[],
				"assetIndex":{"id":"legacy","url":"` + srvURL(r, "/index.json") + `","sha1":"","size":1}
			}`))
		case strings.HasSuffix(r.URL.Path, "index.json"):
			w.Write([]byte(`{"objects":{}}`))
		default:
			w.Write(payload)
		}
	}))
	defer srv.Close()
	// rewrite handler now that URL is known
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch {
		case r.URL.Path == "/ver.json":
			_, _ = w.Write([]byte(`{
				"id":"1.0-test",
				"downloads":{"client":{"url":"` + srv.URL + `/client.jar","sha1":"` + sha + `","size":12}},
				"libraries":[],
				"assetIndex":{"id":"legacy","sha1":"","url":"` + srv.URL + `/index.json","size":2}
			}`))
		case r.URL.Path == "/index.json":
			_, _ = w.Write([]byte(`{"objects":{}}`))
		default:
			_, _ = w.Write(payload)
		}
	})

	dir := t.TempDir()
	err := Install(context.Background(), dir, RemoteVersion{ID: "1.0-test", URL: srv.URL + "/ver.json"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(dir, "versions", "1.0-test", "1.0-test.jar")
	b, err := os.ReadFile(jar)
	if err != nil || string(b) != string(payload) {
		t.Fatalf("jar %q err %v", b, err)
	}
	first := hits
	if err := Install(context.Background(), dir, RemoteVersion{ID: "1.0-test", URL: srv.URL + "/ver.json"}, nil); err != nil {
		t.Fatal(err)
	}
	if hits <= first {
		t.Fatal("expected a second manifest fetch")
	}
}

func srvURL(r *http.Request, path string) string { return "http://" + r.Host + path }

func TestWantNative(t *testing.T) {
	if !wantNative("natives-linux") && !wantNative("natives-windows") && !wantNative("natives-macos") && !wantNative("natives-osx") {
		t.Fatal("this OS should want some native classifier")
	}
}
