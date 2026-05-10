package main

import (
	"testing"
)

func TestBlockCalculateHashStable(t *testing.T) {
	tx := NewCoinbaseTx("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", BlockReward, 1, 0)
	b := NewBlock(3, []*Transaction{tx}, "deadbeef")
	h1 := b.CalculateHash()
	h2 := b.CalculateHash()
	if h1 != h2 {
		t.Fatalf("hash mismatch")
	}
}

func TestBlockIsValidRejectsWrongPrevHash(t *testing.T) {
	tx := NewCoinbaseTx("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", BlockReward, 0, 0)
	b := NewBlock(1, []*Transaction{tx}, "expectedprev")
	b.Nonce = 0
	b.Hash = b.CalculateHash()
	if b.IsValid("wrongprev", 0) {
		t.Fatal("expected invalid prev hash")
	}
	if !b.IsValid("expectedprev", 0) {
		t.Fatal("expected valid with correct prev")
	}
}

func TestBlockMineDifficultyOne(t *testing.T) {
	tx := NewCoinbaseTx("bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", BlockReward, 0, 0)
	b := NewBlock(0, []*Transaction{tx}, "0")
	b.Mine(1)
	if b.Hash[0] != '0' {
		t.Fatalf("hash %s should start with 0 for difficulty 1", b.Hash[:8])
	}
	if !b.IsValid("0", 1) {
		t.Fatal("mined block invalid")
	}
}

func TestBlockIsValidRejectsTamperedHash(t *testing.T) {
	tx := NewCoinbaseTx("cccccccccccccccccccccccccccccccccccccccc", BlockReward, 0, 0)
	b := NewBlock(0, []*Transaction{tx}, "0")
	b.Mine(2)
	b.Hash = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	if b.IsValid("0", 2) {
		t.Fatal("tampered hash should fail")
	}
}
