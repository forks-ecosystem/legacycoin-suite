package net

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"sync/atomic"
	"time"
	"bytes"
	"encoding/binary"

	//"github.com/legacycoin/legacycoin/blockchain"
)

const (
	ProtocolVersion = 70001
	UserAgent       = "/LegacyCoin:0.1.0/"
	PingInterval    = 2 * time.Minute
	HandshakeTimeout = 30 * time.Second
	ReadTimeout      = 5 * time.Minute
)

// Peer represents a connected remote node.
type Peer struct {
	conn        net.Conn
	addr        string
	inbound     bool
	version     int32
	height      int32
	userAgent   string
	services    uint64
	connected   int32 // atomic bool
	lastSeen    time.Time
	nonce       uint64

	chain   *blockchain.Chain
	server  *Server

	sendCh  chan *Message
	quitCh  chan struct{}
	once    sync.Once
}

// newPeer creates a Peer wrapping the given connection.
func newPeer(conn net.Conn, inbound bool, chain *blockchain.Chain, server *Server) *Peer {
	return &Peer{
		conn:     conn,
		addr:     conn.RemoteAddr().String(),
		inbound:  inbound,
		chain:    chain,
		server:   server,
		sendCh:   make(chan *Message, 64),
		quitCh:   make(chan struct{}),
		nonce:    rand.Uint64(),
		lastSeen: time.Now(),
	}
}

// Start begins the send/receive loops for this peer.
func (p *Peer) Start() {
	atomic.StoreInt32(&p.connected, 1)
	go p.sendLoop()
	go p.recvLoop()
	go p.pingLoop()
	// Initiate handshake for outbound connections
	if !p.inbound {
		p.sendVersion()
	}
}

// Stop disconnects this peer and removes it from the server's peer map.
func (p *Peer) Stop() {
	p.once.Do(func() {
		atomic.StoreInt32(&p.connected, 0)
		close(p.quitCh)
		p.conn.Close()
		log.Printf("Peer %s disconnected", p.addr)
		if p.server != nil {
			p.server.removePeer(p)
		}
	})
}

// IsConnected returns true if the peer is active.
func (p *Peer) IsConnected() bool {
	return atomic.LoadInt32(&p.connected) == 1
}

// Send queues a message for sending to this peer.
func (p *Peer) Send(cmd string, payload []byte) {
	select {
	case p.sendCh <- &Message{Command: cmd, Payload: payload}:
	case <-p.quitCh:
	}
}

func (p *Peer) sendLoop() {
	for {
		select {
		case msg := <-p.sendCh:
			p.conn.SetWriteDeadline(time.Now().Add(30 * time.Second))
			if err := WriteMessage(p.conn, msg.Command, msg.Payload); err != nil {
				log.Printf("Peer %s write error: %v", p.addr, err)
				p.Stop()
				return
			}
		case <-p.quitCh:
			return
		}
	}
}

func (p *Peer) recvLoop() {
	defer p.Stop()
	for {
		p.conn.SetReadDeadline(time.Now().Add(ReadTimeout))
		msg, err := ReadMessage(p.conn)
		if err != nil {
			if p.IsConnected() {
				log.Printf("Peer %s read error: %v", p.addr, err)
			}
			return
		}
		p.lastSeen = time.Now()
		p.handleMessage(msg)
	}
}

func (p *Peer) pingLoop() {
        pingTicker := time.NewTicker(PingInterval)
        defer pingTicker.Stop()
        
        syncTicker := time.NewTicker(30 * time.Second) // каждые 30 секунд
        defer syncTicker.Stop()

        for {
                select {
                case <-pingTicker.C:
                        if p.IsConnected() {
                                p.Send(CmdPing, []byte{1, 2, 3, 4})
                                log.Printf("[P2P] %s: PING sent", p.addr)
                        }
                case <-syncTicker.C:
                        if !p.IsConnected() {
                                continue
                        }
                        myHeight := p.chain.BestHeight()
                        // Якщо пір оголосив висоту більше нашої — тягнемо блоки
                        if int64(p.height) > myHeight {
                            log.Printf("[SYNC] %s: requesting blocks (my=%d, peer=%d)", p.addr, myHeight, p.height)
                            p.sendGetBlocks()
                        } else {
                            // Профілактика: навіть якщо висоти рівні, просимо анонси (Inv), 
                            // щоб дізнатися, чи не з'явився блок, про який ми не почули.
                            p.sendGetBlocks() 
                        }
                        // Завжди просимо getblocks, щоб отримати актуальні inv-анонси.
                case <-p.quitCh:
                        return
                }
        }
}
func (p *Peer) handleMessage(msg *Message) {
	switch msg.Command {
	case CmdVersion:
		p.handleVersion(msg.Payload)
	case CmdVerack:
		p.handleVerack()
	case CmdGetBlocks:
		p.handleGetBlocks(msg.Payload)
	case CmdInv:
		p.handleInv(msg.Payload)
	case CmdGetData:
		p.handleGetData(msg.Payload)
	case CmdBlock:
		p.handleBlock(msg.Payload)
	case CmdTx:
		p.handleTx(msg.Payload)
	case CmdGetAddr:
		p.handleGetAddr()
	case CmdAddr:
		p.handleAddr(msg.Payload)
	case CmdPing:
		p.Send(CmdPong, msg.Payload)
	case CmdPong:
		// update latency if needed
                // update latency and trigger sync
                log.Printf("[P2P] %s: received PONG, checking sync", p.addr)
                // После успешного PONG проверяем синхронизацию
                if int64(p.height) > p.chain.BestHeight() {
                        p.sendGetBlocks()
                }
	default:
		log.Printf("Peer %s: unknown command %q", p.addr, msg.Command)
	}
}

// ── Handshake ─────────────────────────────────────────────────────────────────

func (p *Peer) sendVersion() {
	v := &VersionPayload{
		Version:   ProtocolVersion,
		Services:  1,
		Timestamp: time.Now().Unix(),
		Nonce:     p.nonce,
		UserAgent: UserAgent,
		Height:    int32(p.chain.BestHeight()),
	}
	p.Send(CmdVersion, EncodeVersion(v))
}

func (p *Peer) handleVersion(payload []byte) {
	v, err := DecodeVersion(payload)
	if err != nil {
		log.Printf("Peer %s: bad version payload: %v", p.addr, err)
		p.Stop()
		return
	}
	p.version = v.Version
	p.height = v.Height
	p.userAgent = v.UserAgent
	p.services = v.Services

	log.Printf("Peer %s: version=%d height=%d agent=%s",
		p.addr, v.Version, v.Height, v.UserAgent)

	// Send verack
	p.Send(CmdVerack, nil)

	// If inbound, we send our version now
	if p.inbound {
		p.sendVersion()
	}

	// Request blocks if peer is ahead
	if int64(v.Height) > p.chain.BestHeight() {
		p.sendGetBlocks()
	}
}

func (p *Peer) handleVerack() {
	log.Printf("Peer %s: handshake complete", p.addr)
	p.server.onPeerReady(p)
        // ONLY START SYNC HERE
        if int64(p.height) > p.chain.BestHeight() {
            p.sendGetBlocks()
        }
}

// ── Block sync ────────────────────────────────────────────────────────────────

func (p *Peer) sendGetBlocks() {
	bestHash := p.chain.BestHash()
	g := &GetBlocksPayload{
		Version:       uint32(ProtocolVersion),
		LocatorHashes: [][32]byte{bestHash},
	}
	p.Send(CmdGetBlocks, EncodeGetBlocks(g))
}

func (p *Peer) handleGetBlocks(payload []byte) {
    r := bytes.NewReader(payload)

    var version uint32
    if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
        return
    }

    hashCount, err := blockchain.ReadVarInt(r)
    if err != nil {
        return
    }

    // 1. Читаем локаторы от соседа
    locatorHashes := make([]blockchain.TxHash, 0, hashCount)
    for i := uint64(0); i < hashCount; i++ {
        var hash blockchain.TxHash
        if _, err := r.Read(hash[:]); err != nil {
            return
        }
        locatorHashes = append(locatorHashes, hash)
    }

    var hashStop blockchain.TxHash
    r.Read(hashStop[:])

    // 2. ИЩЕМ ОБЩУЮ ТОЧКУ (Intersection)
    // Мы идем по локаторам соседа и проверяем, есть ли у нас такой блок
    var startHeight int64 = -1
    for _, locator := range locatorHashes {
        // ПРЯМАЯ ПРОВЕРКА ГЕНЕЗИСА
        // Если локатор совпадает с нашим генезисом, мы нашли точку опоры
        if locator == p.chain.GenesisHash() {
            startHeight = 1
            break
        }
        height, err := p.chain.GetBlockHeight(locator) // Убедитесь, что метод GetBlockHeight реализован в Chain
        if err == nil && height >= 0 {
            startHeight = height + 1
            break
        }
    }
    // ВАЖНО: Если общих блоков нет, начинаем с 0 (Генезис мы не шлем, шлем блок 1)
    if startHeight == -1 {
        log.Printf("[SYNC] %s: No common blocks found, starting from genesis (height 0)", p.addr)
        startHeight = 1
    }
    // 3. Отправляем блоки, начиная с ОБЩЕЙ ТОЧКИ
    var items []InvVector
    myBestHeight := p.chain.BestHeight()
    
    // Лимит 500 блоков за раз
    end := startHeight + 500
    if end > myBestHeight {
        end = myBestHeight
    }

    // Если мы на одной высоте, слать нечего
    if startHeight > myBestHeight {
        return
    }

    log.Printf("[SYNC] %s: intersection found at height %d, sending up to %d", p.addr, startHeight-1, end)

    for h := startHeight; h <= end; h++ {
        hash, err := p.chain.GetBlockHashAtHeight(h)
        if err != nil {
            continue
        }
        items = append(items, InvVector{Type: InvTypeBlock, Hash: hash})
        if hash == hashStop {
            break
        }
    }

    if len(items) > 0 {
        p.Send(CmdInv, EncodeInv(items))
    }
}
func (p *Peer) handleInv(payload []byte) {
	items, err := DecodeInv(payload)
	if err != nil {
		return
	}
	var want []InvVector
	for _, item := range items {
		switch item.Type {
		case InvTypeBlock:
			hash := blockchain.TxHash(item.Hash)
			if !p.chain.HasBlock(hash) {
				want = append(want, item)
			}
		case InvTypeTx:
			// TODO: mempool check
		}
	}
	if len(want) > 0 {
		p.Send(CmdGetData, EncodeInv(want))
	}
}

func (p *Peer) handleGetData(payload []byte) {
	items, err := DecodeInv(payload)
	if err != nil {
		return
	}
	for _, item := range items {
		switch item.Type {
		case InvTypeBlock:
			hash := blockchain.TxHash(item.Hash)
			block, err := p.chain.GetBlock(hash)
			if err != nil {
				continue
			}
			p.Send(CmdBlock, block.Serialize())
		}
	}
}

func (p *Peer) handleBlock(payload []byte) {
    block, err := blockchain.DeserializeBlock(payload)
    if err != nil {
        log.Printf("Peer %s: invalid block: %v", p.addr, err)
        return
    }

    // 1. Пытаемся добавить блок в цепочку
    if err := p.chain.ProcessBlock(block); err != nil {
        // Если блок уже есть (HasBlock), ProcessBlock обычно возвращает nil или специальную ошибку
        // Если это реальная ошибка валидации — выходим
        log.Printf("Peer %s: block rejected/exists: %v", p.addr, err)
        return
    }

    // 2. Обновляем высоту пира (чтобы syncTicker видел актуальное состояние)
    atomic.StoreInt32(&p.height, int32(p.chain.BestHeight()))

    // 3. Рассылаем анонс (Inv) другим узлам
    p.server.RelayBlock(block, p)

    // 4. Если мы всё еще отстаем от заявленной высоты пира — просим следующий кусок
    myHeight := p.chain.BestHeight()
    if myHeight < int64(p.height) {
        log.Printf("[SYNC] %s: progress %d/%d, requesting more", p.addr, myHeight, p.height)
        p.sendGetBlocks()
    }
}
func (p *Peer) handleTx(payload []byte) {
    // Оборачиваем payload в io.Reader
    tx, err := blockchain.DeserializeTransaction(bytes.NewReader(payload))
    if err != nil {
        log.Printf("Peer %s: invalid tx: %v", p.addr, err)
        return
    }
    p.server.AddToMempool(tx)
}

func (p *Peer) handleGetAddr() {
	addrs := p.server.KnownAddresses()
	if len(addrs) > 0 {
		p.Send(CmdAddr, EncodeAddr(addrs))
	}
}

func (p *Peer) handleAddr(payload []byte) {
	// Store new peer addresses for later connection
	log.Printf("Peer %s: received addr payload (%d bytes)", p.addr, len(payload))
}

// Addr returns the peer's remote address string.
func (p *Peer) Addr() string {
	return p.addr
}

// Height returns the peer's reported chain height.
func (p *Peer) Height() int32 {
	return p.height
}

// String returns a human-readable peer description.
func (p *Peer) String() string {
	dir := "outbound"
	if p.inbound {
		dir = "inbound"
	}
	return fmt.Sprintf("%s (%s) height=%d agent=%s", p.addr, dir, p.height, p.userAgent)
}
