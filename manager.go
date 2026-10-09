package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
)

type Config struct {
	Wallet  string `json:"wallet"`
	Pool    string `json:"pool"`
	Workers int    `json:"workers"`
	Worker  string `json:"worker"`
}

const (
	configFile = "config/miner.json"
	minWorkers = 1
	maxWorkers = 1024
)

func (c Config) Validate() error {
	if strings.TrimSpace(c.Pool) == "" {
		return fmt.Errorf("pool address is required")
	}
	if strings.TrimSpace(c.Wallet) == "" {
		return fmt.Errorf("wallet address is required")
	}
	if c.Workers < minWorkers || c.Workers > maxWorkers {
		return fmt.Errorf("workers must be between %d and %d", minWorkers, maxWorkers)
	}
	return nil
}

func (c *Config) UnmarshalJSON(data []byte) error {
	var raw struct {
		Wallet  string      `json:"wallet"`
		Pool    string      `json:"pool"`
		Worker  string      `json:"worker"`
		Workers interface{} `json:"workers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	n, err := flexWorkers(raw.Workers)
	if err != nil {
		return fmt.Errorf("workers: %w", err)
	}
	c.Wallet = raw.Wallet
	c.Pool = raw.Pool
	c.Worker = raw.Worker
	c.Workers = n
	return nil
}

func flexWorkers(v interface{}) (int, error) {
	switch t := v.(type) {
	case nil:
		return 0, fmt.Errorf("is required")
	case float64:
		n := int(t)
		if float64(n) != t {
			return 0, fmt.Errorf("must be a whole number")
		}
		return n, nil
	case json.Number:
		n, err := t.Int64()
		if err != nil {
			return 0, fmt.Errorf("must be a whole number")
		}
		return int(n), nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, fmt.Errorf("is required")
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			return 0, fmt.Errorf("must be a number")
		}
		return n, nil
	default:
		return 0, fmt.Errorf("must be a number")
	}
}

func defaultConfig() Config {
	return Config{
		Wallet:  "",
		Pool:    "stratum+tcp://127.0.0.1:3333",
		Workers: 4,
		Worker:  "cpu",
	}
}

func loadConfig() Config {
	cfg := defaultConfig()

	data, err := os.ReadFile(configFile)
	if err == nil {
		if err := json.Unmarshal(data, &cfg); err != nil {
			log.Printf("Config: ignoring %s: %v", configFile, err)
			cfg = defaultConfig()
		}
	}

	if v := os.Getenv("WALLET"); v != "" {
		cfg.Wallet = v
	}
	if v := os.Getenv("POOL"); v != "" {
		cfg.Pool = v
	}
	if v := os.Getenv("WORKER"); v != "" {
		cfg.Worker = v
	}
	if v := os.Getenv("WORKERS"); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 {
			cfg.Workers = n
		}
	}

	return cfg
}

func saveConfig(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(configFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".miner-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), configFile)
}

type Manager struct {
	mu      sync.Mutex
	miner   *Miner
	cfg     Config
	running bool
}

func NewManager(cfg Config) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) Config() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.cfg
}

func (m *Manager) Start() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		log.Println("Manager: already running")
		return
	}

	m.miner = NewMiner(m.cfg.Pool, m.cfg.Wallet, m.cfg.Worker, m.cfg.Workers)
	go m.miner.Run()
	m.running = true
	log.Println("Manager: miner started")
}

func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running || m.miner == nil {
		return
	}

	close(m.miner.quit)
	m.miner.sendMu.Lock()
	if m.miner.conn != nil {
		m.miner.conn.Close()
	}
	m.miner.sendMu.Unlock()
	m.running = false
	log.Println("Manager: miner stopped")
}

func (m *Manager) Restart() {
	log.Println("Manager: restarting...")
	m.Stop()
	m.Start()
}

func (m *Manager) UpdateConfig(cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}

	m.mu.Lock()
	m.cfg = cfg
	m.mu.Unlock()

	if err := saveConfig(cfg); err != nil {
		return err
	}

	m.Restart()
	return nil
}

func (m *Manager) Status() map[string]interface{} {
	m.mu.Lock()
	cfg := m.cfg
	running := m.running
	miner := m.miner
	m.mu.Unlock()

	stats := map[string]interface{}{
		"running": running,
		"wallet":  cfg.Wallet,
		"pool":    cfg.Pool,
		"workers": cfg.Workers,
		"worker":  cfg.Worker,
	}

	if miner != nil && running {
		s := miner.Stats()
		for k, v := range s {
			stats[k] = v
		}
	}

	return stats
}
