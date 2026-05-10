package main

import (
	"testing"
)

func TestTransactionIsCoinbase(t *testing.T) {
	cb := NewCoinbaseTx("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", BlockReward, 0, 0)
	if !cb.IsCoinbase() {
		t.Fatal("coinbase")
	}
	tx := &Transaction{
		Inputs:  []TxInput{{TxID: "x", OutIndex: 0}},
		Outputs: []TxOutput{{Amount: 1, Address: "a"}},
	}
	tx.ID = tx.Hash()
	if tx.IsCoinbase() {
		t.Fatal("not coinbase")
	}
}

func TestTransactionHashDeterministic(t *testing.T) {
	tx := NewCoinbaseTx("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", 5.5, 2, 0.1)
	h1 := tx.Hash()
	h2 := tx.Hash()
	if h1 != h2 || tx.ID != h1 {
		t.Fatalf("id/hash")
	}
}

func TestVerifyCoinbaseNoSig(t *testing.T) {
	cb := NewCoinbaseTx("cccccccccccccccccccccccccccccccccccccccc", BlockReward, 3, 0)
	if !cb.Verify() {
		t.Fatal("coinbase verify")
	}
}

func TestNewCoinbaseAddsFeesToOutput(t *testing.T) {
	cb := NewCoinbaseTx("dddddddddddddddddddddddddddddddddddddddd", BlockReward, 1, 2.5)
	if len(cb.Outputs) != 1 {
		t.Fatal("outputs")
	}
	if cb.Outputs[0].Amount != BlockReward+2.5 {
		t.Fatalf("amount %v", cb.Outputs[0].Amount)
	}
}
