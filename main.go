package main

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"math/big"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"legacycoin-miner/crypto"
)

//go:embed static/dashboard.html
var dashboardHTML string

type StratumMessage struct {
	ID     any           `json:"id"`
	Method string        `json:"method,omitempty"`
	Params []interface{} `json:"params,omitempty"`
}

type MiningJob struct {
	JobID     string
	PrevHash  string
	Coinbase1 string
	Coinbase2 string
	Branches  []string
	Version   string
	NBits     string
	NTime     string
	CleanJobs bool
	submitted atomic.Bool
}

type Miner struct {
	poolAddress   string
	walletAddress string
	workerName    string
	workers       int

	conn    net.Conn
	encoder *json.Encoder
	decoder *json.Decoder

	currentJob  atomic.Value
	extranonce1 string

	shareDifficulty float64

	stats  MiningStats
	quit   chan struct{}
	sendMu sync.Mutex
}

type MiningStats struct {
	AcceptedShares int64
	TotalHashes    uint64
	HashRate       int64
	StartTime      time.Time
	lastHashTime   time.Time
}

func main() {
	webAddr := flag.String("web", ":3002", "Web server address")
	flag.Parse()

	cfg := loadConfig()

	if cfg.Wallet == "" {
		log.Fatal("Wallet not set. Set WALLET env var or configure via web UI")
	}

	mgr := NewManager(cfg)
	mgr.Start()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mgr.Status())
	})

	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "GET":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(mgr.Config())
		case "PUT":
			var cfg Config
			if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			if err := mgr.UpdateConfig(cfg); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/api/miner/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case "POST":
			switch r.URL.Path {
			case "/api/miner/start":
				mgr.Start()
				json.NewEncoder(w).Encode(map[string]string{"status": "started"})
			case "/api/miner/stop":
				mgr.Stop()
				json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
			case "/api/miner/restart":
				mgr.Restart()
				json.NewEncoder(w).Encode(map[string]string{"status": "restarted"})
			default:
				http.Error(w, "not found", http.StatusNotFound)
			}
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(dashboardHTML))
	})

	log.Printf("Web UI: http://0.0.0.0%s", *webAddr)
	log.Printf("Config: pool=%s wallet=%s workers=%d", cfg.Pool, cfg.Wallet, cfg.Workers)

	if err := http.ListenAndServe(*webAddr, mux); err != nil {
		log.Fatalf("Web server error: %v", err)
	}
}

func (m *Miner) Stats() map[string]interface{} {
	return map[string]interface{}{
		"poolAddress":    m.poolAddress,
		"walletAddress":  m.walletAddress,
		"workerName":     m.workerName,
		"workers":        m.workers,
		"acceptedShares": atomic.LoadInt64(&m.stats.AcceptedShares),
		"totalHashes":    atomic.LoadUint64(&m.stats.TotalHashes),
		"hashRate":       float64(atomic.LoadInt64(&m.stats.HashRate)),
		"uptimeSeconds":  time.Since(m.stats.StartTime).Seconds(),
	}
}

func NewMiner(pool, wallet, worker string, workers int) *Miner {
	m := &Miner{
		poolAddress:   pool,
		walletAddress: wallet,
		workerName:    worker,
		workers:       workers,
		quit:          make(chan struct{}),
		stats:         MiningStats{StartTime: time.Now(), lastHashTime: time.Now()},
	}
	m.currentJob.Store((*MiningJob)(nil))
	return m
}

func (m *Miner) Run() {
	log.Printf("Legacycoin Miner | Workers: %d", m.workers)

	go m.reportStats()

	for i := 0; i < m.workers; i++ {
		go m.worker(i)
	}

	for {
		if err := m.connect(); err != nil {
			log.Printf("Connect error: %v", err)
			select {
			case <-time.After(5 * time.Second):
				continue
			case <-m.quit:
				return
			}
		}

		m.subscribe()
		m.authorize()

		m.handleMessages()

		m.sendMu.Lock()
		if m.conn != nil {
			m.conn.Close()
			m.conn = nil
		}
		m.sendMu.Unlock()
		m.currentJob.Store((*MiningJob)(nil))

		select {
		case <-m.quit:
			return
		default:
		}

		log.Println("Disconnected, reconnecting in 5s...")
		select {
		case <-time.After(5 * time.Second):
		case <-m.quit:
			return
		}
	}
}

// ====================== NETWORK ======================

func (m *Miner) connect() error {
	m.sendMu.Lock()
	defer m.sendMu.Unlock()

	addr := strings.TrimPrefix(m.poolAddress, "stratum+tcp://")
	if !strings.Contains(addr, ":") {
		addr += ":3333"
	}
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		return err
	}

	tcpConn := conn.(*net.TCPConn)
	tcpConn.SetKeepAlive(true)
	tcpConn.SetKeepAlivePeriod(30 * time.Second)

	if m.conn != nil {
		m.conn.Close()
	}
	m.conn = conn
	m.encoder = json.NewEncoder(conn)
	m.decoder = json.NewDecoder(conn)
	return nil
}

func (m *Miner) subscribe() error {
	return m.send(StratumMessage{ID: 1, Method: "mining.subscribe", Params: []interface{}{"legacycoin-miner/1.0"}})
}

func (m *Miner) authorize() error {
	user := fmt.Sprintf("%s.%s", m.walletAddress, m.workerName)
	return m.send(StratumMessage{ID: 2, Method: "mining.authorize", Params: []interface{}{user, "x"}})
}

func (m *Miner) send(msg interface{}) error {
	m.sendMu.Lock()
	defer m.sendMu.Unlock()
	return m.encoder.Encode(msg)
}

// ====================== MINING ======================

func (m *Miner) worker(id int) {
	var localJob *MiningJob

	for {
		select {
		case <-m.quit:
			return
		default:
		}

		job := m.currentJob.Load().(*MiningJob)
		if job == nil || job == localJob {
			time.Sleep(5 * time.Millisecond)
			continue
		}

		localJob = job
		m.mineJob(id, job)
	}
}

const poolPersonalization = "LegacyCoinPoW"

func (m *Miner) mineJob(workerID int, job *MiningJob) {
	extranonce2 := fmt.Sprintf("%08x", workerID)
	coinbase := m.buildCoinbase(job, extranonce2)
	merkleRoot := m.calculateMerkleRoot(coinbase, job.Branches)
	shareTarget := TargetForDifficulty(m.shareDifficulty)
	netTarget := CompactToBig(bitsToCompact(job.NBits))
	version, _ := hex.DecodeString(job.Version)
	prevHash, _ := hex.DecodeString(job.PrevHash)
	nbits, _ := hex.DecodeString(job.NBits)

	for nonce := uint32(workerID); ; nonce += uint32(m.workers) {
		select {
		case <-m.quit:
			return
		default:
		}

		if m.currentJob.Load().(*MiningJob) != job {
			return
		}

		atomic.AddUint64(&m.stats.TotalHashes, 1)

		ntime := uint32(time.Now().Unix())
		ntimeBytes := make([]byte, 4)
		binary.LittleEndian.PutUint32(ntimeBytes, ntime)

		header := buildHeader(version, prevHash, merkleRoot, ntimeBytes, nbits, nonce)
		hash := crypto.YespowerHash(poolPersonalization, header)

		if HashMeetsTarget(hash[:], shareTarget) {
			m.submitShare(job, extranonce2, ntime, nonce)

			if HashMeetsTarget(hash[:], netTarget) {
				log.Printf("BLOCK FOUND by worker %d", workerID)
				log.Printf("  hash (LE): %x", hash[:])
				log.Printf("  hash (BE): %x", reverseBytes(hash[:]))
				log.Printf("  header: %x", header)
				log.Printf("  nonce: %d", nonce)
			}
		}
	}
}

// ====================== CRYPTO HELPERS ======================

func (m *Miner) buildCoinbase(job *MiningJob, en2 string) []byte {
	c1, _ := hex.DecodeString(job.Coinbase1)
	e1, _ := hex.DecodeString(m.extranonce1)
	e2, _ := hex.DecodeString(en2)
	c2, _ := hex.DecodeString(job.Coinbase2)
	b := append(c1, e1...)
	b = append(b, e2...)
	b = append(b, c2...)
	return b
}

func (m *Miner) calculateMerkleRoot(coinbase []byte, branches []string) []byte {
	root := doubleHashB(coinbase)
	for _, br := range branches {
		b, _ := hex.DecodeString(br)
		root = doubleHashB(append(root, b...))
	}
	return root
}

func buildHeader(v, prev, merkle, ntime, nbits []byte, nonce uint32) []byte {
	buf := new(bytes.Buffer)
	buf.Write(reverseBytes(v))
	buf.Write(reverseBytes(prev))
	buf.Write(merkle)
	buf.Write(ntime)
	buf.Write(reverseBytes(nbits))

	nb := make([]byte, 4)
	binary.LittleEndian.PutUint32(nb, nonce)
	buf.Write(nb)
	return buf.Bytes()
}

func doubleHashB(data []byte) []byte {
	h1 := sha256.Sum256(data)
	h2 := sha256.Sum256(h1[:])
	return h2[:]
}

func bitsToCompact(bitsHex string) uint32 {
	if len(bitsHex) != 8 {
		return 0
	}
	c, _ := strconv.ParseUint(bitsHex, 16, 32)
	return uint32(c)
}

var poolDiff1 = new(big.Int).SetBytes([]byte{
	0x00, 0x00, 0x7f, 0xff, 0xff, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
})

func CompactToBig(compact uint32) *big.Int {
	size := compact >> 24
	word := compact & 0x007fffff
	bn := new(big.Int).SetUint64(uint64(word))
	if size <= 3 {
		bn.Rsh(bn, uint(8*(3-size)))
	} else {
		bn.Lsh(bn, uint(8*(size-3)))
	}
	if compact&0x00800000 != 0 {
		bn.Neg(bn)
	}
	return bn
}

func TargetForDifficulty(diff float64) *big.Int {
	if diff <= 0 || math.IsNaN(diff) || math.IsInf(diff, 0) {
		diff = 1
	}
	// Avoid float precision loss: target = poolDiff1 * 1e9 / uint64(diff * 1e9)
	intDiff := uint64(diff * 1e9)
	if intDiff == 0 {
		intDiff = 1e9
	}
	denom := new(big.Int).SetUint64(intDiff)
	target := new(big.Int).Mul(poolDiff1, new(big.Int).SetUint64(1e9))
	target.Div(target, denom)
	if target.Sign() <= 0 {
		return big.NewInt(1)
	}
	return target
}

func HashMeetsTarget(hash []byte, target *big.Int) bool {
	return HashToBig(hash).Cmp(target) <= 0
}

func HashToBig(hash []byte) *big.Int {
	rev := make([]byte, len(hash))
	for i := range hash {
		rev[i] = hash[len(hash)-1-i]
	}
	return new(big.Int).SetBytes(rev)
}

func reverseBytes(b []byte) []byte {
	r := make([]byte, len(b))
	for i := range b {
		r[i] = b[len(b)-1-i]
	}
	return r
}

// ====================== SUBMIT ======================

func (m *Miner) submitShare(job *MiningJob, en2 string, ntime, nonce uint32) {
	m.send(StratumMessage{
		ID:     3,
		Method: "mining.submit",
		Params: []interface{}{
			fmt.Sprintf("%s.%s", m.walletAddress, m.workerName),
			job.JobID,
			en2,
			fmt.Sprintf("%08x", ntime),
			fmt.Sprintf("%08x", nonce),
		},
	})
}

// ====================== HANDLING ======================

func (m *Miner) handleMessages() {
	for {
		var raw json.RawMessage
		if err := m.decoder.Decode(&raw); err != nil {
			select {
			case <-m.quit:
				return
			default:
				log.Printf("Decode error: %v", err)
				return
			}
		}

		var msg map[string]interface{}
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}

		if id, ok := msg["id"].(float64); ok {
			if id == 1 {
				if resArr, ok := msg["result"].([]interface{}); ok && len(resArr) >= 3 {
					if en1, ok := resArr[1].(string); ok {
						m.extranonce1 = en1
					}
				}
				continue
			}
			if id == 3 {
				if errVal, hasErr := msg["error"]; hasErr && errVal != nil {
					log.Printf("rejected: %v", errVal)
				} else if result, hasResult := msg["result"]; hasResult {
					if b, ok := result.(bool); ok && b {
						atomic.AddInt64(&m.stats.AcceptedShares, 1)
					}
				}
				continue
			}
			continue
		}

		if method, ok := msg["method"].(string); ok {
			switch method {
			case "mining.notify":
				if p, ok := msg["params"].([]interface{}); ok && len(p) >= 9 {
					if job := parseJob(p); job != nil {
						m.currentJob.Store(job)
						log.Printf("job %s", job.JobID)
					}
				}
			case "mining.set_difficulty":
				if p, ok := msg["params"].([]interface{}); ok && len(p) > 0 {
					if d, ok := p[0].(float64); ok {
						m.shareDifficulty = d
					}
				}
			}
		}
	}
}

func parseJob(params []interface{}) *MiningJob {
	job := &MiningJob{
		JobID:     fmt.Sprintf("%v", params[0]),
		PrevHash:  fmt.Sprintf("%v", params[1]),
		Coinbase1: fmt.Sprintf("%v", params[2]),
		Coinbase2: fmt.Sprintf("%v", params[3]),
		NBits:     fmt.Sprintf("%v", params[6]),
		NTime:     fmt.Sprintf("%v", params[7]),
	}
	if cj, ok := params[8].(bool); ok {
		job.CleanJobs = cj
	}
	if m, ok := params[4].([]interface{}); ok {
		job.Branches = make([]string, len(m))
		for i, v := range m {
			job.Branches[i] = fmt.Sprintf("%v", v)
		}
	}
	switch v := params[5].(type) {
	case string:
		job.Version = v
	case float64:
		job.Version = fmt.Sprintf("%08x", uint32(v))
	}
	return job
}

// ====================== STATS ======================

func (m *Miner) reportStats() {
	ticker := time.NewTicker(8 * time.Second)
	defer ticker.Stop()

	var prevHashes uint64
	var prevTime time.Time

	for {
		select {
		case <-ticker.C:
			hashes := atomic.LoadUint64(&m.stats.TotalHashes)
			now := time.Now()

			if prevHashes > 0 {
				elapsed := now.Sub(prevTime).Seconds()
				rate := float64(hashes-prevHashes) / elapsed
				atomic.StoreInt64(&m.stats.HashRate, int64(rate))
			}

			prevHashes = hashes
			prevTime = now

			log.Printf("acc %d  rate %.0f H/s",
				atomic.LoadInt64(&m.stats.AcceptedShares),
				float64(atomic.LoadInt64(&m.stats.HashRate)))
		case <-m.quit:
			return
		}
	}
}

