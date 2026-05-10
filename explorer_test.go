package main

import (
	"testing"
	"time"
)

func TestBlockToDTOCoinbase(t *testing.T) {
	tx := NewCoinbaseTx("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", BlockReward, 0, 0)
	b := &Block{
		Index:        2,
		Timestamp:    time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC).UnixNano(),
		Transactions: []*Transaction{tx},
		PrevHash:     "abc",
		Hash:         "def",
		Nonce:        42,
	}
	d := blockToDTO(b)
	if d.Index != 2 || d.Nonce != 42 || d.PrevHash != "abc" || d.Hash != "def" {
		t.Fatal("header")
	}
	if len(d.Transactions) != 1 || d.Transactions[0].Type != "coinbase" {
		t.Fatal("tx type")
	}
}

func TestBlockToDTODataTx(t *testing.T) {
	tx := &Transaction{
		Inputs:  []TxInput{{TxID: "x", OutIndex: 0}},
		Outputs: []TxOutput{{Amount: 1, Address: "a"}},
		Data:    "hello",
	}
	tx.ID = tx.Hash()
	b := &Block{Index: 0, Timestamp: 0, Transactions: []*Transaction{tx}, PrevHash: "0", Hash: "h", Nonce: 0}
	d := blockToDTO(b)
	if d.Transactions[0].Type != "data" {
		t.Fatal(d.Transactions[0].Type)
	}
}
