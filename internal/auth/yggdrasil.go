package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Yggdrasil authenticates against a classic auth server.
// Mojang shut the public one down; third-party roots (LittleSkin, ely.by,
// authlib-injector) still speak this protocol. Root is the server origin
// without a trailing /authenticate, e.g. https://littleskin.cn/api/yggdrasil
// or the historical https://authserver.mojang.com.
func Yggdrasil(ctx context.Context, root, email, password string) (Session, error) {
	email = strings.TrimSpace(email)
	if email == "" || password == "" {
		return Session{}, fmt.Errorf("帐号邮箱或密码为空")
	}
	root = strings.TrimRight(strings.TrimSpace(root), "/")
	if root == "" {
		return Session{}, fmt.Errorf("没有填写验证服务器")
	}
	body := map[string]any{
		"agent":       map[string]any{"name": "Minecraft", "version": 1},
		"username":    email,
		"password":    password,
		"requestUser": true,
	}
	raw, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, root+"/authenticate", bytes.NewReader(raw))
	if err != nil {
		return Session{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "mclauncher-go/1.0")
	client := &http.Client{Timeout: 25 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return Session{}, fmt.Errorf("登录失败: %w", err)
	}
	defer res.Body.Close()
	buf, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		return Session{}, fmt.Errorf("登录失败 (%d): %s", res.StatusCode, trimErr(buf))
	}
	var parsed struct {
		AccessToken string `json:"accessToken"`
		Selected    struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"selectedProfile"`
	}
	if err := json.Unmarshal(buf, &parsed); err != nil {
		return Session{}, fmt.Errorf("登录响应无法解析")
	}
	if parsed.AccessToken == "" || parsed.Selected.Name == "" {
		return Session{}, fmt.Errorf("登录失败")
	}
	return Session{
		Name:        parsed.Selected.Name,
		UUID:        strings.ReplaceAll(parsed.Selected.ID, "-", ""),
		AccessToken: parsed.AccessToken,
		UserType:    "legacy",
	}, nil
}

func trimErr(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 240 {
		return s[:240]
	}
	if s == "" {
		return "空响应"
	}
	return s
}
