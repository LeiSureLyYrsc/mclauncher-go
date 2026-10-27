package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const authlibLatest = "https://bmclapi2.bangbang93.com/mirrors/authlib-injector/artifact/latest.json"

type authlibMeta struct {
	Version     string `json:"version"`
	DownloadURL string `json:"download_url"`
	Checksums   struct {
		SHA256 string `json:"sha256"`
	} `json:"checksums"`
}

// EnsureAuthlib returns a local authlib-injector jar. preferred is used when
// that file already exists. Otherwise the latest build is fetched from BMCLAPI
// into dest.
func EnsureAuthlib(ctx context.Context, preferred, dest string) (string, error) {
	if p := strings.TrimSpace(preferred); p != "" {
		if st, err := os.Stat(p); err == nil && !st.IsDir() && st.Size() > 0 {
			return p, nil
		}
	}
	if dest == "" {
		return "", fmt.Errorf("没有 Authlib Injector 的保存位置")
	}
	if st, err := os.Stat(dest); err == nil && !st.IsDir() && st.Size() > 0 {
		return dest, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, authlibLatest, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "mclauncher-go/1.0")
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("获取 Authlib Injector 失败: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("获取 Authlib Injector 失败: HTTP %d", res.StatusCode)
	}
	var meta authlibMeta
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&meta); err != nil {
		return "", err
	}
	if meta.DownloadURL == "" {
		return "", fmt.Errorf("Authlib Injector 清单没有下载地址")
	}
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, meta.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "mclauncher-go/1.0")
	res2, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("下载 Authlib Injector 失败: %w", err)
	}
	defer res2.Body.Close()
	if res2.StatusCode >= 300 {
		return "", fmt.Errorf("下载 Authlib Injector 失败: HTTP %d", res2.StatusCode)
	}
	if err := os.MkdirAll(dirOf(dest), 0o755); err != nil {
		return "", err
	}
	tmp := dest + ".part"
	f, err := os.Create(tmp)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(f, h), res2.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return "", err
	}
	if want := strings.ToLower(meta.Checksums.SHA256); want != "" && hex.EncodeToString(h.Sum(nil)) != want {
		os.Remove(tmp)
		return "", fmt.Errorf("Authlib Injector 校验失败")
	}
	if err := os.Rename(tmp, dest); err != nil {
		return "", err
	}
	return dest, nil
}

func dirOf(path string) string {
	i := strings.LastIndexAny(path, `/\`)
	if i <= 0 {
		return "."
	}
	return path[:i]
}
