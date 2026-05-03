package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"
)

type Block struct {
	Index        int
	Timestamp    int64
	Transactions []*Transaction
	PrevHash     string
	Hash         string
	Nonce        int
}

func NewBlock(index int, txs []*Transaction, prevHash string) *Block {
	return &Block{
		Index:        index,
		Timestamp:    time.Now().UnixNano(),
		Transactions: txs,
		PrevHash:     prevHash,
	}
}

func (b *Block) CalculateHash() string {
	txData, _ := json.Marshal(b.Transactions)
	raw := fmt.Sprintf(
		"%d%d%s%s%d", 
		b.Index, 
		b.Timestamp, 
		txData, 
		b.PrevHash, 
		b.Nonce,
	)
	h := sha256.Sum256([]byte(raw))
	
	return hex.EncodeToString(h[:])
}

func (b *Block) Mine(difficulty int) {
	target := ""
	for i := 0; i < difficulty; i++ {
		target += "0"
	}

	for {
		b.Hash = b.CalculateHash()
		if b.Hash[:difficulty] == target {
			fmt.Printf("  ⛏  Block #%d mined  nonce=%-8d  hash=%s...\n", b.Index, b.Nonce, b.Hash[:16])
			return
		}
		b.Nonce++
	}
}

func (b *Block) IsValid(prevHash string, difficulty int) bool {
	if b.PrevHash != prevHash {
		return false
	}
	target := ""
	for i := 0; i < difficulty; i++ {
		target += "0"
	}
	if b.Hash[:difficulty] != target {
		return false
	}
	if b.Hash != b.CalculateHash() {
		return false
	}
	for _, tx := range b.Transactions {
		if !tx.Verify() {
			return false
		}
	}
	return true
}
