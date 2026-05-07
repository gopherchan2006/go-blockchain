package main

import (
	"fmt"
	"sync"
)

type Mempool struct {
	mu  sync.Mutex
	txs []*Transaction
}

func NewMempool() *Mempool {
	return &Mempool{}
}

func (m *Mempool) Add(tx *Transaction) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	spent := make(map[string]map[int]bool)
	for _, existing := range m.txs {
		for _, in := range existing.Inputs {
			if spent[in.TxID] == nil {
				spent[in.TxID] = make(map[int]bool)
			}
			spent[in.TxID][in.OutIndex] = true
		}
	}
	for _, in := range tx.Inputs {
		if spent[in.TxID][in.OutIndex] {
			return fmt.Errorf("double spend: output %s:%d already in mempool", in.TxID, in.OutIndex)
		}
	}
	m.txs = append(m.txs, tx)
	return nil
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
