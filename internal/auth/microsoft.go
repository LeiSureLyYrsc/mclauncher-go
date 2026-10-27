package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Public client id used by the official Minecraft launcher and by HMCL / Prism.
// It is not a secret; Microsoft device-code login requires it.
const minecraftClientID = "00000000402b5328"

// DeviceCode is what the UI shows while polling.
type DeviceCode struct {
	UserCode        string `json:"user_code"`
	DeviceCode      string `json:"device_code"`
	VerificationURI string `json:"verification_uri"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	Message         string `json:"message"`
}

type msToken struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	Error        string `json:"error"`
	ErrorDesc    string `json:"error_description"`
}

// StartMicrosoft begins a device-code login. The caller polls PollMicrosoft.
func StartMicrosoft(ctx context.Context) (DeviceCode, error) {
	form := url.Values{
		"client_id": {minecraftClientID},
		"scope":     {"XboxLive.signin offline_access"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://login.microsoftonline.com/consumers/oauth2/v2.0/devicecode",
		strings.NewReader(form.Encode()))
	if err != nil {
		return DeviceCode{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var dc DeviceCode
	if err := doJSON(ctx, req, &dc); err != nil {
		return DeviceCode{}, err
	}
	if dc.DeviceCode == "" || dc.UserCode == "" {
		return DeviceCode{}, fmt.Errorf("微软没有返回设备码")
	}
	if dc.Interval < 2 {
		dc.Interval = 5
	}
	if dc.VerificationURI == "" {
		dc.VerificationURI = "https://www.microsoft.com/link"
	}
	return dc, nil
}

// PollMicrosoft blocks until the user finishes the browser login, then walks
// Xbox Live → XSTS → Minecraft Services and returns a game session.
func PollMicrosoft(ctx context.Context, dc DeviceCode) (Session, error) {
	deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
	if dc.ExpiresIn <= 0 {
		deadline = time.Now().Add(15 * time.Minute)
	}
	interval := time.Duration(dc.Interval) * time.Second
	for {
		if err := ctx.Err(); err != nil {
			return Session{}, err
		}
		if time.Now().After(deadline) {
			return Session{}, fmt.Errorf("登录超时，请重新获取设备码")
		}
		tok, err := pollToken(ctx, dc.DeviceCode)
		if err != nil {
			return Session{}, err
		}
		if tok.Error == "authorization_pending" {
			select {
			case <-ctx.Done():
				return Session{}, ctx.Err()
			case <-time.After(interval):
				continue
			}
		}
		if tok.Error != "" {
			return Session{}, fmt.Errorf("微软登录失败: %s", tok.ErrorDesc)
		}
		return minecraftFromMS(ctx, tok)
	}
}

func pollToken(ctx context.Context, device string) (msToken, error) {
	form := url.Values{
		"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		"client_id":   {minecraftClientID},
		"device_code": {device},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://login.microsoftonline.com/consumers/oauth2/v2.0/token",
		strings.NewReader(form.Encode()))
	if err != nil {
		return msToken{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var tok msToken
	if err := doJSON(ctx, req, &tok); err != nil {
		return msToken{}, err
	}
	return tok, nil
}

func minecraftFromMS(ctx context.Context, tok msToken) (Session, error) {
	xblBody := map[string]any{
		"Properties": map[string]any{
			"AuthMethod": "RPS",
			"SiteName":   "user.auth.xboxlive.com",
			"RpsTicket":  "d=" + tok.AccessToken,
		},
		"RelyingParty": "http://auth.xboxlive.com",
		"TokenType":    "JWT",
	}
	var xbl struct {
		Token         string `json:"Token"`
		DisplayClaims struct {
			Xui []struct {
				Uhs string `json:"uhs"`
			} `json:"xui"`
		} `json:"DisplayClaims"`
	}
	if err := postJSON(ctx, "https://user.auth.xboxlive.com/user/authenticate", xblBody, &xbl); err != nil {
		return Session{}, fmt.Errorf("Xbox Live: %w", err)
	}
	if xbl.Token == "" || len(xbl.DisplayClaims.Xui) == 0 {
		return Session{}, fmt.Errorf("Xbox Live 没有返回令牌")
	}
	uhs := xbl.DisplayClaims.Xui[0].Uhs
	xstsBody := map[string]any{
		"Properties": map[string]any{
			"SandboxId":  "RETAIL",
			"UserTokens": []string{xbl.Token},
		},
		"RelyingParty": "rp://api.minecraftservices.com/",
		"TokenType":    "JWT",
	}
	var xsts struct {
		Token string `json:"Token"`
		XErr  int64  `json:"XErr"`
	}
	if err := postJSON(ctx, "https://xsts.auth.xboxlive.com/xsts/authorize", xstsBody, &xsts); err != nil {
		return Session{}, fmt.Errorf("XSTS: %w", err)
	}
	if xsts.XErr == 2148916233 {
		return Session{}, fmt.Errorf("这个微软账号没有 Xbox 档案，请先在 xbox.com 创建一个")
	}
	if xsts.Token == "" {
		return Session{}, fmt.Errorf("XSTS 拒绝登录 (XErr %d)", xsts.XErr)
	}
	mcBody := map[string]string{
		"identityToken": fmt.Sprintf("XBL3.0 x=%s;%s", uhs, xsts.Token),
	}
	var mc struct {
		AccessToken string `json:"access_token"`
	}
	if err := postJSON(ctx, "https://api.minecraftservices.com/authentication/login_with_xbox", mcBody, &mc); err != nil {
		return Session{}, fmt.Errorf("Minecraft Services: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.minecraftservices.com/minecraft/profile", nil)
	if err != nil {
		return Session{}, err
	}
	req.Header.Set("Authorization", "Bearer "+mc.AccessToken)
	var profile struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := doJSON(ctx, req, &profile); err != nil {
		return Session{}, fmt.Errorf("没有购买 Minecraft Java 版，或档案尚未创建: %w", err)
	}
	if profile.Name == "" || profile.ID == "" {
		return Session{}, fmt.Errorf("没有购买 Minecraft Java 版，或档案尚未创建")
	}
	return Session{
		Name:         profile.Name,
		UUID:         strings.ReplaceAll(profile.ID, "-", ""),
		AccessToken:  mc.AccessToken,
		UserType:     "msa",
		RefreshToken: tok.RefreshToken,
	}, nil
}

func postJSON(ctx context.Context, url string, body any, dest any) error {
	raw, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	return doJSON(ctx, req, dest)
}

func doJSON(ctx context.Context, req *http.Request, dest any) error {
	req.Header.Set("User-Agent", "mclauncher-go/1.0")
	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	buf, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 && !json.Valid(buf) {
		return fmt.Errorf("HTTP %d", res.StatusCode)
	}
	if dest == nil {
		return nil
	}
	if err := json.Unmarshal(buf, dest); err != nil {
		return fmt.Errorf("HTTP %d: %w", res.StatusCode, err)
	}
	return nil
}
