package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"strconv"

	"github.com/syndtr/goleveldb/leveldb"
)

const (
	Difficulty  = 4
	BlockReward = 10.0
)

type utxoRef struct {
	TxID     string
	OutIndex int
	Amount   float64
}

type Blockchain struct {
	db     *leveldb.DB
	height int
}

func NewBlockchain(dbPath string, minerAddress string) (*Blockchain, error) {
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, err
	}

	bc := &Blockchain{
		db:     db,
		height: -1,
	}

	exists, err := db.Has([]byte("height"), nil)
	if err != nil {
		return nil, err
	}

	if !exists {
		genesis := NewBlock(0, []*Transaction{NewCoinbaseTx(minerAddress, BlockReward, 0, 0)}, "0")
		genesis.Mine(Difficulty)
		err = bc.saveBlock(genesis)
		if err != nil {
			return nil, err
		}
		bc.height = 0
	} else {
		heightData, err := db.Get([]byte("height"), nil)
		if err != nil {
			return nil, err
		}
		h, err := strconv.Atoi(string(heightData))
		if err != nil {
			return nil, err
		}
		bc.height = h
	}

	return bc, nil
}

func (bc *Blockchain) saveBlock(block *Block) error {
	key := []byte(fmt.Sprintf("block:%d", block.Index))
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(block)
	if err != nil {
		return err
	}
	err = bc.db.Put(key, buf.Bytes(), nil)
	if err != nil {
		return err
	}
	return bc.db.Put([]byte("height"), []byte(strconv.Itoa(block.Index)), nil)
}

func (bc *Blockchain) getBlock(index int) (*Block, error) {
	key := []byte(fmt.Sprintf("block:%d", index))
	data, err := bc.db.Get(key, nil)
	if err != nil {
		return nil, err
	}
	var block Block
	dec := gob.NewDecoder(bytes.NewReader(data))
	err = dec.Decode(&block)
	return &block, err
}

func (bc *Blockchain) Close() error {
	return bc.db.Close()
}

func (bc *Blockchain) Height() int {
	return bc.height
}

func sumFees(txs []*Transaction) float64 {
	total := 0.0
	for _, tx := range txs {
		if !tx.IsCoinbase() {
			total += tx.Fee
		}
	}
	return total
}

func (bc *Blockchain) NewBlockTemplate(minerAddress string, txs []*Transaction) (*Block, error) {
	nextHeight := bc.height + 1
	coinbase := NewCoinbaseTx(minerAddress, BlockReward, nextHeight, sumFees(txs))
	all := append([]*Transaction{coinbase}, txs...)
	prevBlock, err := bc.getBlock(bc.height)
	if err != nil {
		return nil, err
	}
	return NewBlock(prevBlock.Index+1, all, prevBlock.Hash), nil
}

func (bc *Blockchain) SubmitBlock(block *Block) error {
	prevBlock, err := bc.getBlock(bc.height)
	if err != nil {
		return err
	}
	if block.Index != bc.height+1 {
		return fmt.Errorf("stale block: expected index %d, got %d", bc.height+1, block.Index)
	}
	if !block.IsValid(prevBlock.Hash, Difficulty) {
		return fmt.Errorf("invalid block")
	}
	if len(block.Transactions) == 0 {
		return fmt.Errorf("block has no transactions")
	}
	coinbaseCount := 0
	for _, tx := range block.Transactions {
		if tx.IsCoinbase() {
			coinbaseCount++
		}
	}
	if coinbaseCount != 1 {
		return fmt.Errorf("invalid coinbase count: %d", coinbaseCount)
	}
	coinbase := block.Transactions[0]
	if !coinbase.IsCoinbase() {
		return fmt.Errorf("first transaction must be coinbase")
	}
	if len(coinbase.Outputs) == 0 {
		return fmt.Errorf("coinbase has no outputs")
	}
	allowedReward := BlockReward + sumFees(block.Transactions[1:])
	if coinbase.Outputs[0].Amount > allowedReward {
		return fmt.Errorf("coinbase exceeds allowed reward")
	}
	spentInBlock := make(map[string]map[int]bool)
	for _, tx := range block.Transactions {
		if tx.IsCoinbase() {
			continue
		}
		if tx.Fee < MinMempoolFee {
			return fmt.Errorf("transaction fee below minimum")
		}
		for _, in := range tx.Inputs {
			if spentInBlock[in.TxID] == nil {
				spentInBlock[in.TxID] = make(map[int]bool)
			}
			if spentInBlock[in.TxID][in.OutIndex] {
				return fmt.Errorf("double spend in block: output %s:%d used twice", in.TxID, in.OutIndex)
			}
			spentInBlock[in.TxID][in.OutIndex] = true
		}
	}
	globalSpent := bc.allSpentOutputs()
	for txID, indices := range spentInBlock {
		for idx := range indices {
			if globalSpent[txID][idx] {
				return fmt.Errorf("double spend: output %s:%d already spent on chain", txID, idx)
			}
		}
	}
	if err := bc.saveBlock(block); err != nil {
		return err
	}
	bc.height = block.Index
	return nil
}

func OpenBlockchain(dbPath string) (*Blockchain, error) {
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, err
	}
	bc := &Blockchain{db: db, height: -1}
	exists, err := db.Has([]byte("height"), nil)
	if err != nil {
		db.Close()
		return nil, err
	}
	if !exists {
		db.Close()
		return nil, fmt.Errorf("blockchain not initialized at %s", dbPath)
	}
	heightData, err := db.Get([]byte("height"), nil)
	if err != nil {
		db.Close()
		return nil, err
	}
	h, err := strconv.Atoi(string(heightData))
	if err != nil {
		db.Close()
		return nil, err
	}
	bc.height = h
	return bc, nil
}

func (bc *Blockchain) AddBlock(txs []*Transaction, minerAddress string) (*Block, error) {
	coinbase := NewCoinbaseTx(minerAddress, BlockReward, bc.height+1, sumFees(txs))
	txs = append([]*Transaction{coinbase}, txs...)

	prevBlock, err := bc.getBlock(bc.height)
	if err != nil {
		return nil, err
	}

	block := NewBlock(prevBlock.Index+1, txs, prevBlock.Hash)
	block.Mine(Difficulty)

	err = bc.saveBlock(block)
	if err != nil {
		return nil, err
	}
	bc.height = block.Index

	return block, nil
}

func (bc *Blockchain) IsValid() bool {
	for i := 1; i <= bc.height; i++ {
		curr, err := bc.getBlock(i)
		if err != nil {
			fmt.Printf("  ✗ Error reading block #%d!\n", i)
			return false
		}

		prev, err := bc.getBlock(i - 1)
		if err != nil {
			fmt.Printf("  ✗ Error reading block #%d!\n", i-1)
			return false
		}

		if !curr.IsValid(prev.Hash, Difficulty) {
			fmt.Printf("  ✗ Block #%d is invalid!\n", curr.Index)
			return false
		}
	}
	return true
}

func (bc *Blockchain) GetBalance(address string) float64 {
	utxos := bc.FindUTXOs(address)
	balance := 0.0
	for _, out := range utxos {
		balance += out.Amount
	}
	return balance
}

func (bc *Blockchain) FindUTXOs(address string) []TxOutput {
	var utxos []TxOutput
	spent := bc.spentOutputs(address)

	for i := 0; i <= bc.height; i++ {
		block, err := bc.getBlock(i)
		if err != nil {
			continue
		}

		for _, tx := range block.Transactions {
			for j, out := range tx.Outputs {
				if out.Address != address {
					continue
				}
				if !spent[tx.ID][j] {
					utxos = append(utxos, out)
				}
			}
		}
	}
	return utxos
}

func (bc *Blockchain) FindSpendableUTXOs(address string, amount float64) (map[string][]int, float64) {
	unspent := make(map[string][]int)
	spent := bc.spentOutputs(address)
	total := 0.0

	for i := 0; i <= bc.height; i++ {
		block, err := bc.getBlock(i)
		if err != nil {
			continue
		}

		for _, tx := range block.Transactions {
			for j, out := range tx.Outputs {
				if out.Address != address || spent[tx.ID][j] {
					continue
				}
				unspent[tx.ID] = append(unspent[tx.ID], j)
				total += out.Amount
				if total >= amount {
					return unspent, total
				}
			}
		}
	}
	return unspent, total
}

func (bc *Blockchain) allSpentOutputs() map[string]map[int]bool {
	spent := make(map[string]map[int]bool)
	for i := 0; i <= bc.height; i++ {
		block, err := bc.getBlock(i)
		if err != nil {
			continue
		}
		for _, tx := range block.Transactions {
			if tx.IsCoinbase() {
				continue
			}
			for _, in := range tx.Inputs {
				if in.TxID == "" {
					continue
				}
				if spent[in.TxID] == nil {
					spent[in.TxID] = make(map[int]bool)
				}
				spent[in.TxID][in.OutIndex] = true
			}
		}
	}
	return spent
}

func (bc *Blockchain) spentOutputs(address string) map[string]map[int]bool {
	spent := make(map[string]map[int]bool)

	for i := 0; i <= bc.height; i++ {
		block, err := bc.getBlock(i)
		if err != nil {
			continue
		}

		for _, tx := range block.Transactions {
			if tx.IsCoinbase() {
				continue
			}

			for _, input := range tx.Inputs {
				if input.TxID == "" {
					continue
				}
				inAddress := pubKeyToAddress(input.PubKey)
				if inAddress == address {
					if spent[input.TxID] == nil {
						spent[input.TxID] = make(map[int]bool)
					}
					spent[input.TxID][input.OutIndex] = true
				}
			}
		}
	}

	return spent
}

func (bc *Blockchain) GetOutputAmount(txID string, outIndex int) float64 {
	for i := 0; i <= bc.height; i++ {
		block, err := bc.getBlock(i)
		if err != nil {
			continue
		}
		for _, tx := range block.Transactions {
			if tx.ID == txID && outIndex < len(tx.Outputs) {
				return tx.Outputs[outIndex].Amount
			}
		}
	}
	return 0
}

func (bc *Blockchain) GetBlocksFrom(from int) []*Block {
	if from < 0 {
		from = 0
	}
	if from > bc.height {
		return nil
	}
	out := make([]*Block, 0, bc.height-from+1)
	for i := from; i <= bc.height; i++ {
		b, err := bc.getBlock(i)
		if err != nil {
			continue
		}
		out = append(out, b)
	}
	return out
}

func (bc *Blockchain) FindSpendableUTXOsWithMempool(address string, amount float64, mp *Mempool) ([]utxoRef, float64) {
	chainSpent := bc.spentOutputs(address)
	mempoolTxs := mp.Peek()

	mempoolSpent := make(map[string]map[int]bool)
	for _, tx := range mempoolTxs {
		for _, in := range tx.Inputs {
			if pubKeyToAddress(in.PubKey) == address {
				if mempoolSpent[in.TxID] == nil {
					mempoolSpent[in.TxID] = make(map[int]bool)
				}
				mempoolSpent[in.TxID][in.OutIndex] = true
			}
		}
	}

	var refs []utxoRef
	total := 0.0

	for i := 0; i <= bc.height; i++ {
		block, err := bc.getBlock(i)
		if err != nil {
			continue
		}
		for _, tx := range block.Transactions {
			for j, out := range tx.Outputs {
				if out.Address != address {
					continue
				}
				if chainSpent[tx.ID][j] || mempoolSpent[tx.ID][j] {
					continue
				}
				refs = append(refs, utxoRef{TxID: tx.ID, OutIndex: j, Amount: out.Amount})
				total += out.Amount
				if total >= amount {
					return refs, total
				}
			}
		}
	}

	for _, tx := range mempoolTxs {
		for j, out := range tx.Outputs {
			if out.Address != address && mempoolSpent[tx.ID][j] {
				continue
			}
			if out.Address != address {
				continue
			}
			refs = append(refs, utxoRef{TxID: tx.ID, OutIndex: j, Amount: out.Amount})
			total += out.Amount
			if total >= amount {
				return refs, total
			}
		}
	}

	return refs, total
}
