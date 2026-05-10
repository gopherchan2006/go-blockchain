package main

import (
	"path/filepath"
	"testing"
)

func testDBPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "chain.db")
}

func TestNewBlockchainGenesisHeight(t *testing.T) {
	p := testDBPath(t)
	bc, err := NewBlockchain(p, "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()
	if bc.Height() != 0 {
		t.Fatalf("height %d", bc.Height())
	}
}

func TestOpenBlockchainReopen(t *testing.T) {
	p := testDBPath(t)
	bc1, err := NewBlockchain(p, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	bc1.Close()
	bc2, err := OpenBlockchain(p)
	if err != nil {
		t.Fatal(err)
	}
	defer bc2.Close()
	if bc2.Height() != 0 {
		t.Fatal("height")
	}
}

func TestOpenBlockchainMissing(t *testing.T) {
	p := filepath.Join(t.TempDir(), "none.db")
	_, err := OpenBlockchain(p)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSubmitBlockSecond(t *testing.T) {
	p := testDBPath(t)
	miner := "cccccccccccccccccccccccccccccccccccccccc"
	bc, err := NewBlockchain(p, miner)
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()
	gen, err := bc.getBlock(0)
	if err != nil {
		t.Fatal(err)
	}
	cb := NewCoinbaseTx(miner, BlockReward, 1, 0)
	b1 := NewBlock(1, []*Transaction{cb}, gen.Hash)
	b1.Mine(Difficulty)
	if err := bc.SubmitBlock(b1); err != nil {
		t.Fatal(err)
	}
	if bc.Height() != 1 {
		t.Fatalf("height %d", bc.Height())
	}
	if bc.GetBalance(miner) != BlockReward*2 {
		t.Fatalf("balance %v", bc.GetBalance(miner))
	}
}

func TestSubmitBlockStaleIndex(t *testing.T) {
	p := testDBPath(t)
	bc, err := NewBlockchain(p, "dddddddddddddddddddddddddddddddddddddddd")
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()
	gen, _ := bc.getBlock(0)
	cb := NewCoinbaseTx("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", BlockReward, 99, 0)
	bad := NewBlock(5, []*Transaction{cb}, gen.Hash)
	bad.Mine(Difficulty)
	if err := bc.SubmitBlock(bad); err == nil {
		t.Fatal("expected stale")
	}
}

func TestBlockchainIsValidChain(t *testing.T) {
	p := testDBPath(t)
	bc, err := NewBlockchain(p, "ffffffffffffffffffffffffffffffffffffffff")
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()
	if !bc.IsValid() {
		t.Fatal("genesis invalid")
	}
}

func TestKnownAddressesIncludesMiner(t *testing.T) {
	p := testDBPath(t)
	miner := "1010101010101010101010101010101010101010"
	bc, err := NewBlockchain(p, miner)
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()
	addrs := bc.KnownAddresses()
	found := false
	for _, a := range addrs {
		if a == miner {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("miner not in known")
	}
}

func TestReplaceTailFromRejectsStartZero(t *testing.T) {
	p := testDBPath(t)
	bc, err := NewBlockchain(p, "2020202020202020202020202020202020202020")
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()
	if err := bc.ReplaceTailFrom(0, []*Block{{Index: 0}}); err == nil {
		t.Fatal("expected invalid start")
	}
}

func TestGetOutputAmount(t *testing.T) {
	p := testDBPath(t)
	miner := "3030303030303030303030303030303030303030"
	bc, err := NewBlockchain(p, miner)
	if err != nil {
		t.Fatal(err)
	}
	defer bc.Close()
	gen, _ := bc.getBlock(0)
	tx := gen.Transactions[0]
	amt := bc.GetOutputAmount(tx.ID, 0)
	if amt != BlockReward {
		t.Fatalf("amt %v", amt)
	}
}
