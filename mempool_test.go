package main

import (
	"path/filepath"
	"testing"
)

func TestMempoolAddRejectLowFee(t *testing.T) {
	m := NewMempool()
	tx := &Transaction{
		Inputs:  []TxInput{{TxID: "a", OutIndex: 0}},
		Outputs: []TxOutput{{Amount: 1, Address: "b"}},
		Fee:     MinMempoolFee / 2,
	}
	tx.ID = tx.Hash()
	if err := m.Add(tx); err == nil {
		t.Fatal("expected fee error")
	}
}

func TestMempoolAddDuplicateID(t *testing.T) {
	m := NewMempool()
	tx := &Transaction{
		Inputs:  []TxInput{{TxID: "a", OutIndex: 0}},
		Outputs: []TxOutput{{Amount: 1, Address: "b"}},
		Fee:     MinMempoolFee,
	}
	tx.ID = tx.Hash()
	if err := m.Add(tx); err != nil {
		t.Fatal(err)
	}
	tx2 := *tx
	if err := m.Add(&tx2); err == nil {
		t.Fatal("expected duplicate")
	}
}

func TestMempoolDoubleSpendInPool(t *testing.T) {
	m := NewMempool()
	tx1 := &Transaction{
		Inputs:  []TxInput{{TxID: "same", OutIndex: 0}},
		Outputs: []TxOutput{{Amount: 1, Address: "x"}},
		Fee:     MinMempoolFee,
	}
	tx1.ID = tx1.Hash()
	if err := m.Add(tx1); err != nil {
		t.Fatal(err)
	}
	tx2 := &Transaction{
		Inputs:  []TxInput{{TxID: "same", OutIndex: 0}},
		Outputs: []TxOutput{{Amount: 1, Address: "y"}},
		Fee:     MinMempoolFee,
	}
	tx2.ID = tx2.Hash()
	if err := m.Add(tx2); err == nil {
		t.Fatal("expected double spend")
	}
}

func TestMempoolRemoveIncluded(t *testing.T) {
	m := NewMempool()
	tx := &Transaction{
		Inputs:  []TxInput{{TxID: "a", OutIndex: 0}},
		Outputs: []TxOutput{{Amount: 1, Address: "b"}},
		Fee:     MinMempoolFee,
	}
	tx.ID = tx.Hash()
	if err := m.Add(tx); err != nil {
		t.Fatal(err)
	}
	m.RemoveIncluded([]*Transaction{tx})
	if m.Size() != 0 {
		t.Fatalf("size %d", m.Size())
	}
}

func TestMempoolPeekSnapshotLen(t *testing.T) {
	m := NewMempool()
	tx1 := &Transaction{Fee: MinMempoolFee, ID: "one"}
	if err := m.Add(tx1); err != nil {
		t.Fatal(err)
	}
	p1 := m.Peek()
	tx2 := &Transaction{Fee: MinMempoolFee, ID: "two"}
	if err := m.Add(tx2); err != nil {
		t.Fatal(err)
	}
	if len(p1) != 1 {
		t.Fatalf("peek len %d", len(p1))
	}
	if len(m.Peek()) != 2 {
		t.Fatal("fresh peek")
	}
}

func TestMempoolFull(t *testing.T) {
	m := NewMempool()
	m.txs = make([]*Transaction, MaxMempoolSize)
	tx := &Transaction{Fee: MinMempoolFee, ID: "last"}
	if err := m.Add(tx); err == nil {
		t.Fatal("expected full")
	}
}

func TestMempoolSaveLoadRoundtrip(t *testing.T) {
	m := NewMempool()
	tx := &Transaction{Fee: MinMempoolFee, ID: "round"}
	if err := m.Add(tx); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "mp.json")
	if err := m.Save(path); err != nil {
		t.Fatal(err)
	}
	m2 := NewMempool()
	if err := m2.Load(path); err != nil {
		t.Fatal(err)
	}
	if m2.Size() != 1 || m2.Peek()[0].ID != "round" {
		t.Fatal("load")
	}
}

func TestMempoolLoadMissingFile(t *testing.T) {
	m := NewMempool()
	path := filepath.Join(t.TempDir(), "missing.json")
	if err := m.Load(path); err != nil {
		t.Fatal(err)
	}
	if m.Size() != 0 {
		t.Fatal("size")
	}
}
