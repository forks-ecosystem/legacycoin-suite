package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
)

type Config struct {
	Wallet  string `json:"wallet"`
	Pool    string `json:"pool"`
	Workers int    `json:"workers"`
	Worker  string `json:"worker"`
}

const configFile = "config.json"

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
		json.Unmarshal(data, &cfg)
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
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configFile, data, 0644)
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
