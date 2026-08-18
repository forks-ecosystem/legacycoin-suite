package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
)

const adminConfigFile = "config/admin.json"

type AdminConfig struct {
	Login         string `json:"login"`
	PasswordHash  string `json:"passwordHash"`
	SessionToken  string `json:"sessionToken"`
}

type AdminAuth struct {
	mu sync.RWMutex
}

func NewAdminAuth() *AdminAuth {
	return &AdminAuth{}
}

func (a *AdminAuth) isConfigured() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	cfg, err := a.loadConfigUnlocked()
	if err != nil {
		return false
	}
	return cfg.Login != "" && cfg.PasswordHash != ""
}

func (a *AdminAuth) loadConfigUnlocked() (*AdminConfig, error) {
	data, err := os.ReadFile(adminConfigFile)
	if err != nil {
		return nil, err
	}
	var cfg AdminConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func (a *AdminAuth) loadConfig() (*AdminConfig, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.loadConfigUnlocked()
}

func (a *AdminAuth) saveConfig(cfg *AdminConfig) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	os.MkdirAll("config", 0755)
	return os.WriteFile(adminConfigFile, data, 0600)
}

func hashPassword(password string) string {
	h := sha256.Sum256([]byte(password))
	return hex.EncodeToString(h[:])
}

func generateToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (a *AdminAuth) Setup(login, password string) (string, error) {
	if a.isConfigured() {
		return "", fmt.Errorf("admin already configured")
	}
	if len(password) < 6 {
		return "", fmt.Errorf("password must be at least 6 characters")
	}

	token := generateToken()
	cfg := &AdminConfig{
		Login:        login,
		PasswordHash: hashPassword(password),
		SessionToken: token,
	}

	if err := a.saveConfig(cfg); err != nil {
		return "", err
	}

	log.Printf("Admin setup: login=%s", login)
	return token, nil
}

func (a *AdminAuth) Login(login, password string) (string, error) {
	cfg, err := a.loadConfig()
	if err != nil {
		return "", fmt.Errorf("admin not configured")
	}

	if cfg.Login != login || cfg.PasswordHash != hashPassword(password) {
		return "", fmt.Errorf("invalid credentials")
	}

	token := generateToken()
	cfg.SessionToken = token
	if err := a.saveConfig(cfg); err != nil {
		return "", err
	}

	log.Printf("Admin login: %s", login)
	return token, nil
}

func (a *AdminAuth) Logout(token string) error {
	cfg, err := a.loadConfig()
	if err != nil {
		return err
	}

	if cfg.SessionToken != token {
		return fmt.Errorf("invalid session")
	}

	cfg.SessionToken = ""
	return a.saveConfig(cfg)
}

func (a *AdminAuth) VerifySession(token string) bool {
	if token == "" {
		return false
	}
	cfg, err := a.loadConfig()
	if err != nil {
		return false
	}
	return cfg.SessionToken == token
}

func (a *AdminAuth) GetConfig() (*AdminConfig, error) {
	return a.loadConfig()
}

func (a *AdminAuth) UpdatePassword(token, newPassword string) error {
	cfg, err := a.loadConfig()
	if err != nil {
		return err
	}
	if cfg.SessionToken != token {
		return fmt.Errorf("invalid session")
	}
	if len(newPassword) < 6 {
		return fmt.Errorf("password must be at least 6 characters")
	}
	cfg.PasswordHash = hashPassword(newPassword)
	return a.saveConfig(cfg)
}

func isNodeRunning() bool {
	cmd := exec.Command("pgrep", "-x", "legacycoind")
	return cmd.Run() == nil
}

func runCLI(args ...string) (string, error) {
	cmd := exec.Command("legacycoin-cli", args...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func getScriptInfo() string {
	paths := []string{
		"/app/_git_LegacyCore.sh",
		"/app/legacybtc-pool/_git_LegacyCore.sh",
	}
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err == nil {
			return string(data)
		}
	}
	return "# Script not found"
}

func GetWalletInfo() map[string]interface{} {
	result := map[string]interface{}{
		"nodeRunning": isNodeRunning(),
	}

	if !result["nodeRunning"].(bool) {
		result["script"] = getScriptInfo()
		return result
	}

	if out, err := runCLI("getwalletinfo"); err == nil {
		var info map[string]interface{}
		if json.Unmarshal([]byte(out), &info) == nil {
			result["walletinfo"] = info
		}
	}

	if out, err := runCLI("getbalance"); err == nil {
		var balance float64
		if json.Unmarshal([]byte(out), &balance) == nil {
			result["balance"] = balance
		}
	}

	if out, err := runCLI("getwalletsummary"); err == nil {
		var summary map[string]interface{}
		if json.Unmarshal([]byte(out), &summary) == nil {
			result["summary"] = summary
		}
	}

	return result
}
