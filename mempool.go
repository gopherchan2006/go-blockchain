package main

import "sync"

type Mempool struct {
	mu  sync.Mutex
	txs []*Transaction
}

func NewMempool() *Mempool {
	return &Mempool{}
}

func (m *Mempool) Add(tx *Transaction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txs = append(m.txs, tx)
}

func (m *Mempool) Flush() []*Transaction {
	m.mu.Lock()
	defer m.mu.Unlock()
	txs := m.txs
	m.txs = nil
	return txs
}

func (m *Mempool) Peek() []*Transaction {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Transaction, len(m.txs))
	copy(out, m.txs)
	return out
}

func (m *Mempool) Size() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.txs)
}
