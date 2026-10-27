// Package download installs versions through BMCLAPI. The 2014 launcher asked
// updatemc.mcsky.net; that mirror is gone, so bangbang93's API is the source.
package download

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/LeiSureLyYrsc/mclauncher-go/internal/launch"
)

type Progress func(done, total int, label string)

type RemoteVersion struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	URL         string `json:"url"`
	ReleaseTime string `json:"releaseTime"`
}

type Manifest struct {
	Latest struct {
		Release  string `json:"release"`
		Snapshot string `json:"snapshot"`
	} `json:"latest"`
	Versions []RemoteVersion `json:"versions"`
}

func FetchManifest(ctx context.Context) (Manifest, string, error) {
	source := sourceFrom(ctx)
	order := []string{source, OtherSource(source)}
	var notes []string
	for i, s := range order {
		var m Manifest
		err := getJSON(WithSource(ctx, s), ManifestURL(s), &m)
		if err == nil && len(m.Versions) > 0 {
			note := ""
			if i > 0 {
				note = "所选下载源失败（" + strings.Join(notes, "；") + "），已改用" + sourceLabel(s)
			}
			return m, note, nil
		}
		if err == nil {
			err = fmt.Errorf("列表是空的")
		}
		notes = append(notes, sourceLabel(s)+" "+err.Error())
		if ctx.Err() != nil {
			break
		}
	}
	return Manifest{}, "", fmt.Errorf("获取列表失败: %s", strings.Join(notes, "；"))
}

func sourceLabel(source string) string {
	if NormalizeSource(source) == SourceBMCLAPI {
		return "BMCLAPI"
	}
	return "官方源"
}

type versionDoc struct {
	ID         string `json:"id"`
	AssetIndex *struct {
		ID   string `json:"id"`
		URL  string `json:"url"`
		SHA1 string `json:"sha1"`
		Size int64  `json:"size"`
	} `json:"assetIndex"`
	Downloads struct {
		Client struct {
			URL  string `json:"url"`
			SHA1 string `json:"sha1"`
			Size int64  `json:"size"`
		} `json:"client"`
	} `json:"downloads"`
	Libraries []libraryDoc `json:"libraries"`
}

type libraryDoc struct {
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Downloads *libDL            `json:"downloads"`
	Natives   map[string]string `json:"natives"`
	Rules     []launch.Rule     `json:"rules"`
}

type libDL struct {
	Artifact    *artifact           `json:"artifact"`
	Classifiers map[string]artifact `json:"classifiers"`
}

type artifact struct {
	Path string `json:"path"`
	URL  string `json:"url"`
	SHA1 string `json:"sha1"`
	Size int64  `json:"size"`
}

type assetIndex struct {
	Objects map[string]struct {
		Hash string `json:"hash"`
		Size int64  `json:"size"`
	} `json:"objects"`
}

// Install downloads the client, libraries allowed on this OS, the asset index,
// and asset objects. Already-correct files (matching sha1) are skipped.
func Install(ctx context.Context, gameDir string, remote RemoteVersion, prog Progress) error {
	if remote.URL == "" || remote.ID == "" {
		return fmt.Errorf("版本信息不完整")
	}
	raw, err := getBytes(ctx, remote.URL)
	if err != nil {
		return fmt.Errorf("下载版本 JSON: %w", err)
	}
	var doc versionDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		return err
	}
	if doc.ID == "" {
		doc.ID = remote.ID
	}
	verDir := filepath.Join(gameDir, "versions", doc.ID)
	if err := os.MkdirAll(verDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(verDir, doc.ID+".json"), raw, 0o644); err != nil {
		return err
	}

	var jobs []job
	if doc.Downloads.Client.URL != "" {
		jobs = append(jobs, job{
			URL:  doc.Downloads.Client.URL,
			SHA1: doc.Downloads.Client.SHA1,
			Path: filepath.Join(verDir, doc.ID+".jar"),
			Name: doc.ID + ".jar",
		})
	}
	for _, lib := range doc.Libraries {
		if !launch.Allowed(lib.Rules, nil) {
			continue
		}
		jobs = append(jobs, libraryJobs(gameDir, lib)...)
	}

	if doc.AssetIndex != nil && doc.AssetIndex.URL != "" {
		idxPath := filepath.Join(gameDir, "assets", "indexes", doc.AssetIndex.ID+".json")
		if err := downloadOne(ctx, job{URL: doc.AssetIndex.URL, SHA1: doc.AssetIndex.SHA1, Path: idxPath, Name: "asset index"}); err != nil {
			return err
		}
		b, err := os.ReadFile(idxPath)
		if err != nil {
			return err
		}
		var index assetIndex
		if err := json.Unmarshal(b, &index); err != nil {
			return err
		}
		for _, obj := range index.Objects {
			if len(obj.Hash) < 2 {
				continue
			}
			jobs = append(jobs, job{
				URL:  "https://resources.download.minecraft.net/" + obj.Hash[:2] + "/" + obj.Hash,
				SHA1: obj.Hash,
				Path: filepath.Join(gameDir, "assets", "objects", obj.Hash[:2], obj.Hash),
				Name: "assets/" + obj.Hash[:8],
			})
		}
	}
	return runJobs(ctx, jobs, prog)
}

type job struct {
	URL, SHA1, Path, Name string
}

func runJobs(ctx context.Context, jobs []job, prog Progress) error {
	if prog == nil {
		prog = func(int, int, string) {}
	}
	var mu sync.Mutex
	done := 0
	total := len(jobs)
	prog(0, total, "准备下载")
	sem := make(chan struct{}, 8)
	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	for _, j := range jobs {
		wg.Add(1)
		sem <- struct{}{}
		go func(j job) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := downloadOne(ctx, j); err != nil {
				select {
				case errCh <- fmt.Errorf("%s: %w", j.Name, err):
					cancel()
				default:
				}
				return
			}
			mu.Lock()
			done++
			n := done
			mu.Unlock()
			prog(n, total, j.Name)
		}(j)
	}
	wg.Wait()
	select {
	case err := <-errCh:
		return err
	default:
		prog(total, total, "完成")
		return nil
	}
}

func downloadOne(ctx context.Context, j job) error {
	if j.SHA1 != "" {
		if ok, _ := fileSHA1(j.Path, j.SHA1); ok {
			return nil
		}
	} else if st, err := os.Stat(j.Path); err == nil && st.Size() > 0 {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(j.Path), 0o755); err != nil {
		return err
	}
	body, err := openGET(ctx, j.URL)
	if err != nil {
		return err
	}
	defer body.Close()
	tmp := j.Path + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	h := sha1.New()
	if _, err := io.Copy(io.MultiWriter(f, h), body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if j.SHA1 != "" && hex.EncodeToString(h.Sum(nil)) != strings.ToLower(j.SHA1) {
		os.Remove(tmp)
		return fmt.Errorf("校验失败")
	}
	return os.Rename(tmp, j.Path)
}

func fileSHA1(path, want string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()
	h := sha1.New()
	if _, err := io.Copy(h, f); err != nil {
		return false, err
	}
	return hex.EncodeToString(h.Sum(nil)) == strings.ToLower(want), nil
}

func getJSON(ctx context.Context, url string, dest any) error {
	b, err := getBytes(ctx, url)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, dest)
}

func libraryJobs(gameDir string, lib libraryDoc) []job {
	var jobs []job
	add := func(rel, rawURL, sha, name string) {
		if rel == "" {
			return
		}
		if rawURL == "" {
			base := "https://libraries.minecraft.net/"
			if lib.URL != "" {
				base = strings.TrimRight(lib.URL, "/") + "/"
			}
			rawURL = base + rel
		}
		jobs = append(jobs, job{
			URL:  rawURL,
			SHA1: sha,
			Path: filepath.Join(gameDir, "libraries", filepath.FromSlash(rel)),
			Name: name,
		})
	}
	if lib.Downloads != nil && lib.Downloads.Artifact != nil && lib.Downloads.Artifact.Path != "" {
		a := lib.Downloads.Artifact
		add(a.Path, a.URL, a.SHA1, filepath.Base(a.Path))
	} else if rel := MavenPath(lib.Name); rel != "" {
		add(rel, "", "", filepath.Base(rel))
	}
	if lib.Downloads != nil {
		for key, a := range lib.Downloads.Classifiers {
			if a.Path == "" || !wantNative(key) {
				continue
			}
			add(a.Path, a.URL, a.SHA1, filepath.Base(a.Path))
		}
	}
	if cls := nativeClassifier(lib.Natives); cls != "" && lib.Name != "" {
		rel := MavenPath(lib.Name + ":" + cls)
		if rel != "" && !hasPath(jobs, rel) {
			add(rel, "", "", filepath.Base(rel))
		}
	}
	return jobs
}

func hasPath(jobs []job, rel string) bool {
	for _, j := range jobs {
		if strings.HasSuffix(filepath.ToSlash(j.Path), rel) {
			return true
		}
	}
	return false
}

func nativeClassifier(natives map[string]string) string {
	if len(natives) == 0 {
		return ""
	}
	key := runtime.GOOS
	switch key {
	case "darwin":
		key = "osx"
	case "windows", "linux":
	default:
		key = "linux"
	}
	return natives[key]
}

func wantNative(classifier string) bool {
	c := strings.ToLower(classifier)
	switch runtime.GOOS {
	case "windows":
		return strings.Contains(c, "natives-windows")
	case "darwin":
		return strings.Contains(c, "natives-macos") || strings.Contains(c, "natives-osx")
	default:
		return strings.Contains(c, "natives-linux")
	}
}
