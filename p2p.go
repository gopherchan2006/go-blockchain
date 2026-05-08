package main

import (
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type P2PMessage struct {
	Type       string         `json:"type"`
	From       string         `json:"from,omitempty"`
	Peers      []string       `json:"peers,omitempty"`
	Height     int            `json:"height,omitempty"`
	FromHeight int            `json:"fromHeight,omitempty"`
	Blocks     []*Block       `json:"blocks,omitempty"`
	Tx         *Transaction   `json:"tx,omitempty"`
	Block      *Block         `json:"block,omitempty"`
	TxID       string         `json:"txID,omitempty"`
	BlockHash  string         `json:"blockHash,omitempty"`
}

type Peer struct {
	addr string
	conn net.Conn
	enc  *json.Encoder
	mu   sync.Mutex
}

type PeerManager struct {
	node       *Node
	listenAddr string

	mu    sync.RWMutex
	peers map[string]*Peer

	seenMu      sync.RWMutex
	seenTx      map[string]time.Time
	seenBlock   map[string]time.Time
	closed      chan struct{}
	listener    net.Listener
	initialPeer []string
}

func NewPeerManager(node *Node, listenAddr string, peers []string) *PeerManager {
	return &PeerManager{
		node:        node,
		listenAddr:  normalizeAddr(listenAddr),
		peers:       make(map[string]*Peer),
		seenTx:      make(map[string]time.Time),
		seenBlock:   make(map[string]time.Time),
		closed:      make(chan struct{}),
		initialPeer: peers,
	}
}

func (pm *PeerManager) Start() error {
	ln, err := net.Listen("tcp", pm.listenAddr)
	if err != nil {
		return err
	}
	pm.listener = ln
	go pm.acceptLoop()
	for _, addr := range pm.initialPeer {
		addr := normalizeAddr(addr)
		if addr == "" || addr == pm.listenAddr {
			continue
		}
		go pm.Connect(addr)
	}
	go pm.gcSeen()
	go pm.maintainConnectivity()
	return nil
}

func (pm *PeerManager) Close() error {
	select {
	case <-pm.closed:
		return nil
	default:
		close(pm.closed)
	}
	if pm.listener != nil {
		_ = pm.listener.Close()
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	for _, p := range pm.peers {
		_ = p.conn.Close()
	}
	pm.peers = make(map[string]*Peer)
	return nil
}

func (pm *PeerManager) acceptLoop() {
	for {
		conn, err := pm.listener.Accept()
		if err != nil {
			select {
			case <-pm.closed:
				return
			default:
			}
			continue
		}
		go pm.registerConn(conn, "")
	}
}

func (pm *PeerManager) Connect(addr string) {
	addr = normalizeAddr(addr)
	if addr == "" || addr == pm.listenAddr {
		return
	}
	if pm.hasPeer(addr) {
		return
	}
	conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
	if err != nil {
		return
	}
	pm.registerConn(conn, addr)
}

func (pm *PeerManager) registerConn(conn net.Conn, expectedAddr string) {
	remote := normalizeAddr(conn.RemoteAddr().String())
	addr := remote
	if expectedAddr != "" {
		addr = normalizeAddr(expectedAddr)
	}
	peer := &Peer{addr: addr, conn: conn, enc: json.NewEncoder(conn)}
	pm.mu.Lock()
	if old, ok := pm.peers[addr]; ok {
		pm.mu.Unlock()
		_ = old.conn.Close()
		pm.mu.Lock()
	}
	pm.peers[addr] = peer
	pm.mu.Unlock()
	_ = pm.sendTo(peer, P2PMessage{Type: "hello", From: pm.listenAddr})
	_ = pm.sendTo(peer, P2PMessage{Type: "get_peers", From: pm.listenAddr})
	_ = pm.sendTo(peer, P2PMessage{Type: "get_height", From: pm.listenAddr})
	dec := json.NewDecoder(conn)
	for {
		var msg P2PMessage
		if err := dec.Decode(&msg); err != nil {
			break
		}
		pm.handleMessage(peer, msg)
	}
	pm.removePeer(peer.addr)
}

func (pm *PeerManager) handleMessage(peer *Peer, msg P2PMessage) {
	switch msg.Type {
	case "hello":
		if msg.From != "" {
			pm.rekeyPeer(peer.addr, normalizeAddr(msg.From), peer)
		}
	case "get_peers":
		_ = pm.sendTo(peer, P2PMessage{Type: "peers", Peers: pm.PeerList(), From: pm.listenAddr})
	case "peers":
		for _, a := range msg.Peers {
			a = normalizeAddr(a)
			if a == "" || a == pm.listenAddr || pm.hasPeer(a) {
				continue
			}
			go pm.Connect(a)
		}
	case "get_height":
		pm.node.mu.Lock()
		h := pm.node.bc.Height()
		pm.node.mu.Unlock()
		_ = pm.sendTo(peer, P2PMessage{Type: "height", Height: h, From: pm.listenAddr})
	case "height":
		pm.node.mu.Lock()
		local := pm.node.bc.Height()
		pm.node.mu.Unlock()
		if msg.Height > local {
			_ = pm.sendTo(peer, P2PMessage{Type: "get_blocks", FromHeight: local + 1, From: pm.listenAddr})
		}
	case "get_blocks":
		if msg.FromHeight < 0 {
			msg.FromHeight = 0
		}
		pm.node.mu.Lock()
		blocks := pm.node.bc.GetBlocksRange(msg.FromHeight, 256)
		pm.node.mu.Unlock()
		_ = pm.sendTo(peer, P2PMessage{Type: "blocks", Blocks: blocks, From: pm.listenAddr})
	case "blocks":
		pm.node.mu.Lock()
		for _, b := range msg.Blocks {
			if b == nil {
				continue
			}
			if b.Index <= pm.node.bc.Height() {
				continue
			}
			if err := pm.node.bc.SubmitBlock(b); err != nil {
				break
			}
		}
		pm.node.mu.Unlock()
	case "new_tx":
		if msg.Tx == nil {
			return
		}
		tx := msg.Tx
		if tx.ID == "" {
			tx.ID = tx.Hash()
		}
		if !tx.Verify() {
			return
		}
		if pm.markTxSeen(tx.ID) {
			return
		}
		if err := pm.node.mempool.Add(tx); err != nil {
			return
		}
		pm.node.hub.Broadcast("new_tx", tx.ID)
		pm.BroadcastTx(tx, peer.addr)
	case "new_block":
		if msg.Block == nil {
			return
		}
		block := msg.Block
		if block.Hash == "" {
			return
		}
		if pm.markBlockSeen(block.Hash) {
			return
		}
		pm.node.mu.Lock()
		err := pm.node.bc.SubmitBlock(block)
		if err == nil {
			pm.node.mempool.RemoveIncluded(block.Transactions)
			pm.node.template = nil
		}
		pm.node.mu.Unlock()
		if err != nil {
			return
		}
		pm.node.hub.Broadcast("new_block", fmt.Sprintf("%d", block.Index))
		pm.BroadcastBlock(block, peer.addr)
	}
}

func (pm *PeerManager) sendTo(peer *Peer, msg P2PMessage) error {
	peer.mu.Lock()
	defer peer.mu.Unlock()
	return peer.enc.Encode(msg)
}

func (pm *PeerManager) BroadcastTx(tx *Transaction, skip string) {
	if tx == nil {
		return
	}
	pm.markTxSeen(tx.ID)
	msg := P2PMessage{Type: "new_tx", Tx: tx, TxID: tx.ID, From: pm.listenAddr}
	pm.broadcast(msg, normalizeAddr(skip))
}

func (pm *PeerManager) BroadcastBlock(block *Block, skip string) {
	if block == nil {
		return
	}
	pm.markBlockSeen(block.Hash)
	msg := P2PMessage{Type: "new_block", Block: block, BlockHash: block.Hash, From: pm.listenAddr}
	pm.broadcast(msg, normalizeAddr(skip))
}

func (pm *PeerManager) broadcast(msg P2PMessage, skip string) {
	pm.mu.RLock()
	list := make([]*Peer, 0, len(pm.peers))
	for addr, p := range pm.peers {
		if addr == skip {
			continue
		}
		list = append(list, p)
	}
	pm.mu.RUnlock()
	for _, p := range list {
		_ = pm.sendTo(p, msg)
	}
}

func (pm *PeerManager) PeerList() []string {
	pm.mu.RLock()
	out := make([]string, 0, len(pm.peers)+1)
	out = append(out, pm.listenAddr)
	for addr := range pm.peers {
		out = append(out, addr)
	}
	pm.mu.RUnlock()
	sort.Strings(out)
	return out
}

func (pm *PeerManager) hasPeer(addr string) bool {
	pm.mu.RLock()
	_, ok := pm.peers[addr]
	pm.mu.RUnlock()
	return ok
}

func (pm *PeerManager) removePeer(addr string) {
	pm.mu.Lock()
	if p, ok := pm.peers[addr]; ok {
		_ = p.conn.Close()
		delete(pm.peers, addr)
	}
	pm.mu.Unlock()
}

func (pm *PeerManager) rekeyPeer(oldAddr, newAddr string, peer *Peer) {
	if newAddr == "" || newAddr == oldAddr {
		return
	}
	pm.mu.Lock()
	delete(pm.peers, oldAddr)
	peer.addr = newAddr
	pm.peers[newAddr] = peer
	pm.mu.Unlock()
}

func (pm *PeerManager) markTxSeen(id string) bool {
	if id == "" {
		return false
	}
	pm.seenMu.Lock()
	_, exists := pm.seenTx[id]
	pm.seenTx[id] = time.Now()
	pm.seenMu.Unlock()
	return exists
}

func (pm *PeerManager) markBlockSeen(hash string) bool {
	if hash == "" {
		return false
	}
	pm.seenMu.Lock()
	_, exists := pm.seenBlock[hash]
	pm.seenBlock[hash] = time.Now()
	pm.seenMu.Unlock()
	return exists
}

func (pm *PeerManager) gcSeen() {
	tick := time.NewTicker(2 * time.Minute)
	defer tick.Stop()
	for {
		select {
		case <-pm.closed:
			return
		case <-tick.C:
			limit := time.Now().Add(-10 * time.Minute)
			pm.seenMu.Lock()
			for k, v := range pm.seenTx {
				if v.Before(limit) {
					delete(pm.seenTx, k)
				}
			}
			for k, v := range pm.seenBlock {
				if v.Before(limit) {
					delete(pm.seenBlock, k)
				}
			}
			pm.seenMu.Unlock()
		}
	}
}

func (pm *PeerManager) maintainConnectivity() {
	tick := time.NewTicker(5 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-pm.closed:
			return
		case <-tick.C:
			for _, addr := range pm.initialPeer {
				addr = normalizeAddr(addr)
				if addr == "" || addr == pm.listenAddr || pm.hasPeer(addr) {
					continue
				}
				go pm.Connect(addr)
			}
		}
	}
}

func normalizeAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.Count(addr, ":") == 0 {
			return ""
		}
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	if _, err := strconv.Atoi(port); err != nil {
		return ""
	}
	return net.JoinHostPort(host, port)
}
