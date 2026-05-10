package main

import (
	"path/filepath"
	"testing"
)

func TestNewTransactionSignVerify(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bc.db")
	w1, err := NewWallet()
	if err != nil {
		t.Fatal(err)
	}
	w2, err := NewWallet()
	if err != nil {
		t.Fatal(err)
	}
	bc, err := NewBlockchain(p, w1.Address())
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()
	tx, err := NewTransaction(w1, w2.Address(), 1.0, bc)
	if err != nil {
		t.Fatal(err)
	}
	if !tx.Verify() {
		t.Fatal("verify")
	}
	if tx.Fee != 0 {
		t.Fatalf("fee %v", tx.Fee)
	}
}
