package main

import (
	"testing"
)

func TestNewWalletAddressLength(t *testing.T) {
	w, err := NewWallet()
	if err != nil {
		t.Fatal(err)
	}
	addr := w.Address()
	if len(addr) != 40 {
		t.Fatalf("addr len %d", len(addr))
	}
}

func TestWalletPublicKeyBytesLength(t *testing.T) {
	w, err := NewWallet()
	if err != nil {
		t.Fatal(err)
	}
	pk := w.PublicKeyBytes()
	if len(pk) != 64 {
		t.Fatalf("pk len %d", len(pk))
	}
}

func TestPubKeyToAddressMatchesWallet(t *testing.T) {
	w, err := NewWallet()
	if err != nil {
		t.Fatal(err)
	}
	a1 := w.Address()
	a2 := pubKeyToAddress(w.PublicKeyBytes())
	if a1 != a2 {
		t.Fatal("address mismatch")
	}
}
