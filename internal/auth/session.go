// Package auth covers the three login paths the original launcher grew into:
// offline name (the 2014 default), Yggdrasil password login (Mojang, now also
// LittleSkin-style third-party roots), and Microsoft device-code login, which
// replaced Mojang passwords in 2022.
package auth

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
)

// Session is what the game process needs. Password never leaves the login call.
type Session struct {
	Name         string
	UUID         string // 32 hex chars, no dashes
	AccessToken  string
	UserType     string // legacy | msa
	RefreshToken string
	Offline      bool
}

func (s Session) UUIDDashed() string {
	u := strings.ReplaceAll(s.UUID, "-", "")
	if len(u) != 32 {
		return s.UUID
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s", u[0:8], u[8:12], u[12:16], u[16:20], u[20:32])
}

func (s Session) PropertiesJSON() string {
	if s.Offline || s.AccessToken == "" {
		return "{}"
	}
	return "{}"
}

// OfflineUUID is the Minecraft offline-mode UUID: UUIDv3 of "OfflinePlayer:"+name.
func OfflineUUID(name string) string {
	sum := md5.Sum([]byte("OfflinePlayer:" + name))
	sum[6] = (sum[6] & 0x0f) | 0x30
	sum[8] = (sum[8] & 0x3f) | 0x80
	return hex.EncodeToString(sum[:])
}

func Offline(name string) (Session, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Session{}, fmt.Errorf("没有填写游戏名字")
	}
	if strings.ContainsAny(name, " \t\r\n") {
		return Session{}, fmt.Errorf("离线名字不能包含空格")
	}
	return Session{
		Name:        name,
		UUID:        OfflineUUID(name),
		AccessToken: "0",
		UserType:    "legacy",
		Offline:     true,
	}, nil
}
