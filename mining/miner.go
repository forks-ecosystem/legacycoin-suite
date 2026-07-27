package mining

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
	"math/rand"
    "legacycoin-miner/crypto"
//	"github.com/legacycoin/legacycoin/blockchain"
//	lccrypto "github.com/legacycoin/legacycoin/crypto"
//	"github.com/legacycoin/legacycoin/wallet"
)

// BlockFoundCallback is called when a valid block is mined.
type BlockFoundCallback func(*blockchain.Block)

// Miner runs CPU mining across multiple goroutines.
type Miner struct {
	chain    *blockchain.Chain
	wallet   *wallet.Wallet
	threads  int
	stopCh   chan struct{}
	wg       sync.WaitGroup
	hashRate int64 // atomic: hashes per second

	OnBlockFound BlockFoundCallback
}

// NewMiner creates a new miner.
func NewMiner(chain *blockchain.Chain, w *wallet.Wallet, threads int) *Miner {
	return &Miner{
		chain:   chain,
		wallet:  w,
		threads: threads,
		stopCh:  make(chan struct{}),
	}
}

// Start begins mining on all configured threads.
func (m *Miner) Start() {
	log.Printf("Miner starting with %d thread(s)", m.threads)
	go m.hashRateLogger()
	for i := 0; i < m.threads; i++ {
		m.wg.Add(1)
		go m.mineWorker(i)
	}
	m.wg.Wait()
}

// Stop signals all mining goroutines to halt.
func (m *Miner) Stop() {
	close(m.stopCh)
	m.wg.Wait()
	log.Println("Miner stopped")
}

// HashRate returns the current hash rate (hashes/sec) across all threads.
func (m *Miner) HashRate() int64 {
	return atomic.LoadInt64(&m.hashRate)
}

func (m *Miner) mineWorker(id int) {
	defer m.wg.Done()
	log.Printf("Mining thread %d started", id)

	for {
		select {
		case <-m.stopCh:
			return
		default:
		}

		block := m.buildTemplate()
		if block == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		if block != nil {
		    block.Header.Nonce = uint32(id * 1000000) 
		    //found := m.searchNonce(block)
		}
		found := m.searchNonce(block)
		if found {
			hash, err := block.Hash()
			if err == nil {
				log.Printf("Thread %d found block! nonce=%d hash=%s", id, block.Header.Nonce, hash)
				if m.OnBlockFound != nil {
					m.OnBlockFound(block)
				}
				// Submit to chain
				if err := m.chain.ProcessBlock(block); err != nil {
					log.Printf("Mined block rejected: %v", err)
				}
			}
		}
	}
}

func (m *Miner) buildTemplate() *blockchain.Block {
	pubKeyHash, err := m.wallet.DefaultPubKeyHash()
	if err != nil {
		log.Printf("Miner: wallet error: %v", err)
		return nil
	}
	coinbaseScript := blockchain.P2PKHScript(pubKeyHash)
	prevHash := m.chain.BestHash()
	bits := m.chain.CurrentBits()
	height := m.chain.BestHeight() + 1
	mempoolTxs := m.chain.Mempool.TxsSortedByFee()
	// Cap at 500 txs per block
	if len(mempoolTxs) > 500 {
		mempoolTxs = mempoolTxs[:500]
	}
	return blockchain.NewBlockTemplate(prevHash, bits, height, coinbaseScript, mempoolTxs)
}

// searchNonce increments nonce until PoW is satisfied, the chain tip changes,
// or the stop signal is received. Returns true if a valid nonce was found.
func (m *Miner) searchNonce(block *blockchain.Block) bool {
	prevHash := m.chain.BestHash()
	var localHashes int64
//my-add
	//for nonce := uint32(0); ; nonce++ {
	for nonce := uint32(rand.Uint32()); ; nonce++ {
		select {
		case <-m.stopCh:
			return false
		default:
		}

		block.Header.Nonce = nonce

		// Every 10k hashes: flush rate counter, update timestamp, check tip
		if nonce%10_000 == 0 && nonce != 0 {
			atomic.AddInt64(&m.hashRate, localHashes)
			localHashes = 0
			if m.chain.BestHash() != prevHash {
				return false // new block arrived — rebuild template
			}
			block.Header.Time = uint32(time.Now().Unix())
		}

		hash, err := lccrypto.YespowerHash(block.Header.Serialize())
		localHashes++
		if err != nil {
			continue
		}
		if blockchain.CheckProofOfWork(blockchain.TxHash(hash), block.Header.Bits) {
			atomic.AddInt64(&m.hashRate, localHashes)
			return true
		}

		// Nonce wrapped around — exhausted 32-bit space, signal rebuild
		if nonce == 0xFFFFFFFF {
			atomic.AddInt64(&m.hashRate, localHashes)
			return false
		}
	}
}

func (m *Miner) hashRateLogger() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	lastSnapshot := atomic.LoadInt64(&m.hashRate)
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			current := atomic.LoadInt64(&m.hashRate)
			delta := current - lastSnapshot
			lastSnapshot = current
			if delta < 0 {
				delta = 0 // counter reset or overflow guard
			}
			log.Printf("Hash rate: %d H/s", delta/10)
		}
	}
}
