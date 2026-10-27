package download

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type sourceKey struct{}

// WithSource selects official or BMCLAPI for every download in this context.
func WithSource(ctx context.Context, source string) context.Context {
	return context.WithValue(ctx, sourceKey{}, NormalizeSource(source))
}

func sourceFrom(ctx context.Context) string {
	s, _ := ctx.Value(sourceKey{}).(string)
	if s == "" {
		return SourceBMCLAPI
	}
	return NormalizeSource(s)
}

// preferIPv4 dials IPv4 first. Domestic networks often black-hole IPv6, and Go
// will otherwise wait on the AAAA address until the request times out.
func preferIPv4(ctx context.Context, network, addr string) (net.Conn, error) {
	d := &net.Dialer{Timeout: 12 * time.Second, KeepAlive: 30 * time.Second}
	if network == "tcp" {
		if c, err := d.DialContext(ctx, "tcp4", addr); err == nil {
			return c, nil
		}
		return d.DialContext(ctx, "tcp6", addr)
	}
	return d.DialContext(ctx, network, addr)
}

var httpClient = &http.Client{
	Timeout: 90 * time.Second,
	Transport: &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		DialContext:           preferIPv4,
		TLSHandshakeTimeout:   12 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		IdleConnTimeout:       30 * time.Second,
		MaxIdleConns:          16,
		ForceAttemptHTTP2:     true,
	},
}

func getBytes(ctx context.Context, rawURL string) ([]byte, error) {
	rc, err := openGET(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	return io.ReadAll(io.LimitReader(rc, 64<<20))
}

// openGET downloads rawURL with the context's source. BMCLAPI answers with a
// 302 to a volunteer node; if that node is unreachable the request is repeated
// so another node is assigned, then the official Mojang URL is tried.
func openGET(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	source := sourceFrom(ctx)
	primary := Rewrite(source, rawURL)
	rc, err := openRetry(ctx, primary, source == SourceBMCLAPI)
	if err == nil {
		return rc, nil
	}
	if source != SourceBMCLAPI || !netErr(err) {
		return nil, err
	}
	official := Official(primary)
	if official == "" || official == primary {
		return nil, err
	}
	rc, err2 := openRetry(ctx, official, false)
	if err2 != nil {
		return nil, fmt.Errorf("%v；官方源 %w", err, err2)
	}
	return rc, nil
}

func openRetry(ctx context.Context, rawURL string, retryNode bool) (io.ReadCloser, error) {
	tries := 1
	if retryNode {
		tries = 3
	}
	var last error
	for i := 0; i < tries; i++ {
		rc, err := doGet(ctx, rawURL)
		if err == nil {
			return rc, nil
		}
		last = err
		if !netErr(err) && !rateLimited(err) {
			return nil, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(time.Duration(i+1) * 500 * time.Millisecond):
		}
	}
	return nil, last
}

func doGet(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "mclauncher-go/1.0")
	res, err := httpClient.Do(req)
	if err != nil {
		host := rawURL
		if req.URL != nil && req.URL.Host != "" {
			host = req.URL.Host
		}
		return nil, fmt.Errorf("连接 %s 失败: %w", host, err)
	}
	if res.StatusCode == http.StatusTooManyRequests || res.StatusCode == http.StatusServiceUnavailable || res.StatusCode >= 300 {
		host := ""
		if res.Request != nil && res.Request.URL != nil {
			host = res.Request.URL.Host
		}
		res.Body.Close()
		return nil, fmt.Errorf("HTTP %d %s", res.StatusCode, host)
	}
	return res.Body, nil
}

func netErr(err error) bool {
	if err == nil {
		return false
	}
	var nerr net.Error
	return errors.As(err, &nerr) || strings.Contains(err.Error(), "连接 ")
}

func rateLimited(err error) bool {
	return err != nil && (strings.Contains(err.Error(), "HTTP 429") || strings.Contains(err.Error(), "HTTP 503"))
}
