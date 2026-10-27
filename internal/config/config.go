// Package config stores launcher settings in the same place the 2014
// mclauncher.cfg lived conceptually: a small file next to the user's data,
// not the registry.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

const FileName = "mclauncher.cfg"

// Config is the persisted launcher state. Passwords are never written.
type Config struct {
	PlayerName      string `json:"playerName"`
	JavaPath        string `json:"javaPath"`
	MaxMemoryMB     int    `json:"maxMemoryMB"`
	MemoryAuto      bool   `json:"memoryAuto"`
	Debug           bool   `json:"debug"`
	GameDir         string `json:"gameDir"`
	SelectedVersion string `json:"selectedVersion"`
	IsolateGame     bool   `json:"isolateGame"`
	// AuthMode: offline | yggdrasil | microsoft
	AuthMode      string `json:"authMode"`
	YggdrasilRoot string `json:"yggdrasilRoot"`
	Email         string `json:"email"`
	AccessToken   string `json:"accessToken,omitempty"`
	RefreshToken  string `json:"refreshToken,omitempty"`
	UUID          string `json:"uuid,omitempty"`
	AuthName      string `json:"authName,omitempty"`
	AuthlibAgent  string `json:"authlibAgent,omitempty"`
	BgMode        string `json:"bgMode"` // random | none
	// Skin is sunset, sky, or default. The files are the 2014 launcher's own pictures.
	Skin     string `json:"skin"`
	FirstRun bool   `json:"firstRun"`
}

func Default(gameDir string) Config {
	return Config{
		PlayerName:    "Player",
		MaxMemoryMB:   1024,
		MemoryAuto:    true,
		GameDir:       gameDir,
		AuthMode:      "offline",
		YggdrasilRoot: "https://authserver.mojang.com",
		BgMode:        "random",
		Skin:          "sunset",
		FirstRun:      true,
	}
}

func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "mclauncher"), nil
}

func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, FileName), nil
}

func Load() (Config, error) {
	path, err := Path()
	if err != nil {
		return Config{}, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		home, _ := os.UserHomeDir()
		cfg := Default(filepath.Join(home, ".minecraft"))
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	home, _ := os.UserHomeDir()
	cfg := Default(filepath.Join(home, ".minecraft"))
	if err := json.Unmarshal(b, &cfg); err != nil {
		return Config{}, err
	}
	if cfg.MaxMemoryMB < 256 {
		cfg.MaxMemoryMB = 1024
	}
	if cfg.AuthMode == "" {
		cfg.AuthMode = "offline"
	}
	if cfg.YggdrasilRoot == "" {
		cfg.YggdrasilRoot = "https://authserver.mojang.com"
	}
	if cfg.Skin == "" {
		cfg.Skin = "sunset"
	}
	if cfg.GameDir == "" {
		cfg.GameDir = filepath.Join(home, ".minecraft")
	}
	return cfg, nil
}

func Save(cfg Config) error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(filepath.Join(dir, FileName), b, 0o600)
}
