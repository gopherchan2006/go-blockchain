package main

import (
	"encoding/json"
	"fmt"
	"os"
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

func (m *Mempool) Save(path string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(m.txs)
}

func (m *Mempool) Load(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	var txs []*Transaction
	if err := json.NewDecoder(f).Decode(&txs); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txs = txs
	return nil
}

func (m *Mempool) RemoveIncluded(blockTxs []*Transaction) {
	if len(blockTxs) == 0 {
		return
	}
	included := make(map[string]bool, len(blockTxs))
	for _, tx := range blockTxs {
		if tx != nil && tx.ID != "" {
			included[tx.ID] = true
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	next := m.txs[:0]
	for _, tx := range m.txs {
		if tx == nil || included[tx.ID] {
			continue
		}
		next = append(next, tx)
	}
	m.txs = next
}
