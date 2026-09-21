package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
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
	if len(password) < 3 {
		return "", fmt.Errorf("password must be at least 3 characters")
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
	if len(newPassword) < 3 {
		return fmt.Errorf("password must be at least 3 characters")
	}
	cfg.PasswordHash = hashPassword(newPassword)
	return a.saveConfig(cfg)
}

func rpcEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func rpcCall(method string, params []interface{}) (json.RawMessage, error) {
	body, err := json.Marshal(map[string]interface{}{
		"jsonrpc": "1.0", "id": 1, "method": method, "params": params,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, rpcEnv("LEGACYCOIN_RPC_URL", "http://127.0.0.1:19556"), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/plain")
	req.SetBasicAuth(rpcEnv("LEGACYCOIN_RPC_USER", "coin"), rpcEnv("LEGACYCOIN_RPC_PASS", "coin"))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("rpc http %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var rr struct {
		Result json.RawMessage `json:"result"`
		Error  *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rr); err != nil {
		return nil, err
	}
	if rr.Error != nil {
		return nil, fmt.Errorf("rpc %s: %s", method, rr.Error.Message)
	}
	return rr.Result, nil
}

func rpcFetch(params ...interface{}) (json.RawMessage, error) {
	return rpcCall("getwalletsummary", params)
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
		"nodeRunning": false,
	}

	if _, err := rpcCall("getblockcount", []interface{}{}); err != nil {
		result["script"] = getScriptInfo()
		return result
	}
	result["nodeRunning"] = true

	if r, err := rpcCall("getbalance", []interface{}{}); err == nil {
		var balance float64
		if json.Unmarshal(r, &balance) == nil {
			result["balance"] = balance
		}
	}

	if r, err := rpcFetch(); err == nil {
		var s struct {
			Spendable        int64             `json:"spendable"`
			Immature         int64             `json:"immature"`
			AddressByHash    map[string]string `json:"address_by_pubkey_hash"`
			Wallet           *struct {
				ClassicKeys int `json:"classic_keys"`
			} `json:"wallet"`
			SpendableOutputs []json.RawMessage `json:"spendable_outputs"`
			ImmatureOutputs  []json.RawMessage `json:"immature_outputs"`
		}
		if json.Unmarshal(r, &s) == nil {
			result["summary"] = map[string]interface{}{
				"balance":             float64(s.Spendable) / 1e8,
				"unconfirmed_balance": float64(0),
				"immature_balance":    float64(s.Immature) / 1e8,
			}

			walletinfo := map[string]interface{}{
				"address": dominantWalletAddress(s.SpendableOutputs, s.ImmatureOutputs, s.AddressByHash),
				"label":   "mining",
			}
			if s.Wallet != nil {
				walletinfo["keypoolsize"] = s.Wallet.ClassicKeys
			}
			walletinfo["txcount"] = countWalletTxids(s.SpendableOutputs, s.ImmatureOutputs)
			result["walletinfo"] = walletinfo
		}
	}

	return result
}

func dominantWalletAddress(spendable, immature []json.RawMessage, byHash map[string]string) string {
	counts := map[string]int{}
	for _, list := range [][]json.RawMessage{spendable, immature} {
		for _, o := range list {
			var e struct {
				Address string `json:"address"`
			}
			if json.Unmarshal(o, &e) == nil && e.Address != "" {
				counts[e.Address]++
			}
		}
	}
	best := ""
	bestCount := 0
	for addr, n := range counts {
		if n > bestCount || (n == bestCount && addr < best) {
			best, bestCount = addr, n
		}
	}
	if best != "" {
		return best
	}
	keys := make([]string, 0, len(byHash))
	for k := range byHash {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	return byHash[keys[0]]
}

func countWalletTxids(lists ...[]json.RawMessage) int {
	set := map[string]struct{}{}
	for _, list := range lists {
		for _, o := range list {
			var e struct {
				TxID string `json:"txid"`
			}
			if json.Unmarshal(o, &e) == nil && e.TxID != "" {
				set[e.TxID] = struct{}{}
			}
		}
	}
	return len(set)
}
