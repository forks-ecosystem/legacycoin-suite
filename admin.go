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
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const adminConfigFile = "config/admin.json"

type AdminConfig struct {
	Login        string `json:"login"`
	PasswordHash string `json:"passwordHash"`
	SessionToken string `json:"sessionToken"`
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
	return rpcCallWallet(method, params, "")
}

// rpcCallWallet runs an RPC against one named node wallet. Address-based send
// RPCs route to the owning wallet on the node, but balance/summary reads still
// need the header to inspect a non-default wallet such as "mainer".
func rpcCallWallet(method string, params []interface{}, wallet string) (json.RawMessage, error) {
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
	if wallet != "" {
		req.Header.Set("X-LegacyCoin-Wallet", wallet)
	}
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
			Spendable     int64             `json:"spendable"`
			Immature      int64             `json:"immature"`
			AddressByHash map[string]string `json:"address_by_pubkey_hash"`
			Wallet        *struct {
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

			defaultAddress := dominantWalletAddress(s.SpendableOutputs, s.ImmatureOutputs, s.AddressByHash)
			walletinfo := map[string]interface{}{
				"address": defaultAddress,
				"label":   "mining",
			}
			if s.Wallet != nil {
				walletinfo["keypoolsize"] = s.Wallet.ClassicKeys
			}
			walletinfo["txcount"] = countWalletTxids(s.SpendableOutputs, s.ImmatureOutputs)
			if defaultAddress != "" {
				if r, err := rpcCall("getaddressbalance", []interface{}{defaultAddress}); err == nil {
					var ab struct {
						Balance int64 `json:"balance_base_units"`
					}
					if json.Unmarshal(r, &ab) == nil {
						walletinfo["balance"] = float64(ab.Balance) / 1e8
					}
				}
			}
			result["walletinfo"] = walletinfo
		}
	}

	if wallets := listWalletBalances(); len(wallets) > 0 {
		result["wallets"] = wallets
	}

	if worker := workerWalletInfo(); worker != nil {
		result["workerinfo"] = worker
	}

	return result
}

// walletOwningAddress returns the node wallet name that tracks addr. Sending
// from an address in a named wallet needs both the owning wallet name (for the
// request header) and the node's sendfromaddress routing, so the suite works
// against older nodes that still default every send to the default wallet.
func walletOwningAddress(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	raw, err := rpcCall("listwallets", []interface{}{})
	if err != nil {
		return ""
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return ""
	}
	for _, name := range names {
		r, err := rpcCallWallet("getwalletsummary", []interface{}{}, name)
		if err != nil {
			continue
		}
		var s struct {
			PubKeyHashByAddress map[string]string `json:"pubkey_hash_by_address"`
		}
		if err := json.Unmarshal(r, &s); err != nil {
			continue
		}
		if _, ok := s.PubKeyHashByAddress[addr]; ok {
			return name
		}
	}
	return ""
}

type sendFeeEstimate struct {
	Fee          float64 `json:"fee"`
	FeeBaseUnits int64   `json:"fee_base_units"`
	MinFee       float64 `json:"min_fee"`
	MinFeeUnits  int64   `json:"min_fee_base_units"`
	FeeRate      float64 `json:"feerate"`
	Inputs       int     `json:"inputs"`
	Size         int64   `json:"size"`
	Spendable    float64 `json:"spendable"`
	Required     float64 `json:"required"`
}

type walletUTXO struct {
	AmountBaseUnits int64  `json:"amount_base_units"`
	Spendable       bool   `json:"spendable"`
	ScriptPubKey    string `json:"scriptPubKey"`
}

// estimateInputSize mirrors the node's transaction sizing for the two address
// families it can spend. Hybrid inputs carry an ML-DSA signature and public
// key, so using the classic P2PKH size for them would underpay the relay fee by
// a large margin.
func estimateInputSize(scriptHex string) int64 {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(scriptHex)), "00a914") {
		return 5418
	}
	// 149 covers the longest DER signature (72-byte sig + sighash byte, plus
	// its push opcode) rather than the usual 148-byte case.
	return 149
}

func nodeFeeRatePerKB() int64 {
	raw, err := rpcCall("estimatefee", []interface{}{6})
	if err != nil {
		return 0
	}
	var est struct {
		FeeRate float64 `json:"feerate"`
	}
	if err := json.Unmarshal(raw, &est); err != nil || est.FeeRate <= 0 {
		return 0
	}
	return int64(math.Ceil(est.FeeRate * 1e8))
}

func nodeMinRelayFeePerKB() int64 {
	raw, err := rpcCall("getmempoolinfo", []interface{}{})
	if err == nil {
		var info struct {
			MinRelayFeeKB int64 `json:"minrelayfeekb"`
		}
		if json.Unmarshal(raw, &info) == nil && info.MinRelayFeeKB > 0 {
			return info.MinRelayFeeKB
		}
	}
	return 1000
}

// estimateSendFee recommends a fee for a send and mirrors the node's coin
// selection order, so the estimated input count (and therefore transaction
// size) matches what sendfromaddress will really sign. Without this the admin
// form kept submitting the old 1000-sat default, which is below the minimum
// relay fee as soon as a send needs more than a handful of inputs.
func estimateSendFee(from string, amount float64) (*sendFeeEstimate, error) {
	from = strings.TrimSpace(from)
	if amount <= 0 {
		return nil, fmt.Errorf("amount must be greater than zero")
	}
	wallet := ""
	params := []interface{}{0, 9999999}
	if from != "" {
		wallet = walletOwningAddress(from)
		if wallet == "" {
			return nil, fmt.Errorf("source address is not owned by any loaded wallet")
		}
		params = append(params, []string{from})
	}
	raw, err := rpcCallWallet("listunspent", params, wallet)
	if err != nil {
		return nil, err
	}
	var utxos []walletUTXO
	if err := json.Unmarshal(raw, &utxos); err != nil {
		return nil, err
	}

	minRelay := nodeMinRelayFeePerKB()
	feerate := nodeFeeRatePerKB()
	if feerate < minRelay {
		feerate = minRelay
	}

	amountSat := int64(math.Round(amount * 1e8))
	var spendable int64
	for _, u := range utxos {
		if u.Spendable && u.AmountBaseUnits > 0 {
			spendable += u.AmountBaseUnits
		}
	}
	if spendable < amountSat {
		return nil, fmt.Errorf("insufficient spendable balance: available %s LBTC, required %s LBTC",
			formatLBTC(spendable), formatLBTC(amountSat))
	}

	fee := minRelay
	var inputs int
	var size int64
	for i := 0; i < 12; i++ {
		inputs = 0
		var total int64
		var inputSize int64
		for _, u := range utxos {
			if !u.Spendable || u.AmountBaseUnits <= 0 {
				continue
			}
			inputs++
			total += u.AmountBaseUnits
			inputSize += estimateInputSize(u.ScriptPubKey)
			if total >= amountSat+fee {
				break
			}
		}
		if inputs == 0 {
			return nil, fmt.Errorf("source address has no spendable UTXOs")
		}
		if total < amountSat+fee {
			return nil, fmt.Errorf("insufficient spendable balance: available %s LBTC, required %s LBTC",
				formatLBTC(spendable), formatLBTC(amountSat+fee))
		}
		outputs := int64(2)
		if total == amountSat+fee {
			outputs = 1
		}
		// 10-byte header, 34-byte output, plus a small margin for CompactSize
		// prefixes and DER signature length variance.
		size = 10 + inputSize + outputs*34 + 32
		next := (size*feerate + 999) / 1000
		minFee := (size*minRelay + 999) / 1000
		if next < minFee {
			next = minFee
		}
		if next == fee {
			break
		}
		fee = next
	}

	minFee := (size*minRelay + 999) / 1000
	return &sendFeeEstimate{
		Fee:          float64(fee) / 1e8,
		FeeBaseUnits: fee,
		MinFee:       float64(minFee) / 1e8,
		MinFeeUnits:  minFee,
		FeeRate:      float64(feerate) / 1e8,
		Inputs:       inputs,
		Size:         size,
		Spendable:    float64(spendable) / 1e8,
		Required:     float64(amountSat+fee) / 1e8,
	}, nil
}

func formatLBTC(baseUnits int64) string {
	return fmt.Sprintf("%.8f", float64(baseUnits)/1e8)
}

// listWalletBalances returns one row per node wallet. The admin panel used to
// show only the default wallet, so coins sitting in a named wallet (for
// example "mainer") looked missing even though the node could see them.
// getbalance is used instead of getwalletsummary because the summary serializes
// every UTXO and the panel polls this endpoint regularly.
func listWalletBalances() []map[string]interface{} {
	raw, err := rpcCall("listwallets", []interface{}{})
	if err != nil {
		return nil
	}
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		r, err := rpcCallWallet("getbalance", []interface{}{}, name)
		if err != nil {
			continue
		}
		var balance float64
		if err := json.Unmarshal(r, &balance); err != nil {
			continue
		}
		out = append(out, map[string]interface{}{
			"name":    name,
			"balance": balance,
		})
	}
	return out
}

func workerWalletInfo() map[string]interface{} {
	addr := strings.TrimSpace(loadConfig().Wallet)
	if addr == "" {
		return nil
	}
	info := map[string]interface{}{
		"address":     addr,
		"confirmed":   0.0,
		"unconfirmed": 0.0,
		"immature":    0.0,
	}
	if r, err := rpcCall("getaddressbalance", []interface{}{addr}); err == nil {
		var ab struct {
			Balance int64 `json:"balance_base_units"`
		}
		if json.Unmarshal(r, &ab) == nil {
			info["confirmed"] = float64(ab.Balance) / 1e8
		}
	}
	return info
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
