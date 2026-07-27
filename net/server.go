package net

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"
    "legacycoin-miner/crypto"
	//"github.com/legacycoin/legacycoin/blockchain"
	//lcwallet "github.com/legacycoin/legacycoin/wallet"
)

const (
	MaxInboundPeers  = 125
	MaxOutboundPeers = 8
	ConnectRetry     = 30 * time.Second
)

// Server is the P2P network server.
type Server struct {
	chain    *blockchain.Chain
	wallet   *lcwallet.Wallet
	port     int
	listener net.Listener

	mu       sync.RWMutex
	peers    map[string]*Peer

	addPeerCh  chan string
	quitCh     chan struct{}
	once       sync.Once
}

// NewServer creates a new P2P server.
func NewServer(chain *blockchain.Chain, wallet *lcwallet.Wallet, port int) *Server {
	return &Server{
		chain:     chain,
		wallet:    wallet,
		port:      port,
		peers:     make(map[string]*Peer),
		addPeerCh: make(chan string, 32),
		quitCh:    make(chan struct{}),
	}
}

// Start begins listening for inbound connections and manages outbound connections.
func (s *Server) Start() {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.port))
	if err != nil {
		log.Fatalf("P2P listen error: %v", err)
	}
	log.Printf("P2P server listening on port %d", s.port)

	go s.connectManager()
	s.acceptLoop()
}

// Stop shuts down the P2P server.
func (s *Server) Stop() {
	s.once.Do(func() {
		close(s.quitCh)
		if s.listener != nil {
			s.listener.Close()
		}
		s.mu.Lock()
		for _, p := range s.peers {
//			p.Stop()
                    // ПРИНУДИТЕЛЬНО закрываем сокеты пиров, чтобы выбить их из recvLoop
                    if p.conn != nil {
                        p.conn.Close()
                    }
		}
		s.mu.Unlock()
	})
}

// AddPeer queues an outbound connection to the given address.
func (s *Server) AddPeer(addr string) {
	select {
	case s.addPeerCh <- addr:
	default:
	}
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			select {
			case <-s.quitCh:
				return
			default:
				log.Printf("Accept error: %v", err)
				continue
			}
		}
		s.mu.RLock()
		inboundCount := s.countInbound()
		s.mu.RUnlock()
		if inboundCount >= MaxInboundPeers {
			conn.Close()
			continue
		}
		go s.handleInbound(conn)
	}
}

func (s *Server) handleInbound(conn net.Conn) {
	peer := newPeer(conn, true, s.chain, s)
	s.registerPeer(peer)
	peer.Start()
}

func (s *Server) connectManager() {
	for {
		select {
		case addr := <-s.addPeerCh:
			go s.connectToPeer(addr)
		case <-s.quitCh:
			return
		}
	}
}

func (s *Server) connectToPeer(addr string) {
	log.Printf("Connecting to peer %s", addr)
	conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
	if err != nil {
		log.Printf("Failed to connect to %s: %v", addr, err)
		return
	}
	peer := newPeer(conn, false, s.chain, s)
	s.registerPeer(peer)
	peer.Start()
}

func (s *Server) registerPeer(peer *Peer) {
	s.mu.Lock()
	s.peers[peer.Addr()] = peer
	s.mu.Unlock()
	log.Printf("Peer registered: %s", peer.Addr())
}

func (s *Server) removePeer(peer *Peer) {
	s.mu.Lock()
	delete(s.peers, peer.Addr())
	s.mu.Unlock()
}

func (s *Server) onPeerReady(peer *Peer) {
	log.Printf("Peer ready: %s", peer)
}

func (s *Server) countInbound() int {
	var n int
	for _, p := range s.peers {
		if p.inbound {
			n++
		}
	}
	return n
}

// RelayBlock broadcasts a block to all peers except the sender.
func (s *Server) RelayBlock(block *blockchain.Block, except *Peer) {
	hash, err := block.Header.Hash()
	if err != nil {
		hash = block.Header.HashFallback()
	}
	inv := []InvVector{{Type: InvTypeBlock, Hash: hash}}
	payload := EncodeInv(inv)

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.peers {
		if p == except || !p.IsConnected() {
			continue
		}
		p.Send(CmdInv, payload)
	}
}

// RelayTransaction broadcasts a transaction to all peers.
func (s *Server) RelayTransaction(tx *blockchain.Transaction) {
	hash := tx.Hash()
	inv := []InvVector{{Type: InvTypeTx, Hash: hash}}
	payload := EncodeInv(inv)

	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, p := range s.peers {
		if !p.IsConnected() {
			continue
		}
		p.Send(CmdInv, payload)
	}
}

// AddToMempool adds a transaction to the local mempool.
func (s *Server) AddToMempool(tx *blockchain.Transaction) {
	if err := blockchain.ValidateTransaction(tx); err != nil {
		return
	}
	s.chain.Mempool.Add(tx, s.chain) //nolint:errcheck
}

// MempoolTxs returns all transactions in the mempool, sorted by fee rate.
func (s *Server) MempoolTxs() []*blockchain.Transaction {
	return s.chain.Mempool.TxsSortedByFee()
}

// Mempool returns the underlying chain mempool.
func (s *Server) Mempool() *blockchain.Mempool {
	return s.chain.Mempool
}

// PeerCount returns the number of connected peers.
func (s *Server) PeerCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.peers)
}

// ConnectedPeers returns a snapshot of connected peers.
func (s *Server) ConnectedPeers() []*Peer {
	s.mu.RLock()
	defer s.mu.RUnlock()
	peers := make([]*Peer, 0, len(s.peers))
	for _, p := range s.peers {
		if p.IsConnected() {
			peers = append(peers, p)
		}
	}
	return peers
}

// KnownAddresses returns peer addresses to share.
func (s *Server) KnownAddresses() []NetAddr {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var addrs []NetAddr
	for _, p := range s.peers {
		if !p.IsConnected() {
			continue
		}
		tcpAddr, ok := p.conn.RemoteAddr().(*net.TCPAddr)
		if !ok {
			continue
		}
		var ip [16]byte
		copy(ip[12:], tcpAddr.IP.To4())
		addrs = append(addrs, NetAddr{
			Timestamp: uint32(time.Now().Unix()),
			Services:  1,
			IP:        ip,
			Port:      uint16(tcpAddr.Port),
		})
	}
	return addrs
}
