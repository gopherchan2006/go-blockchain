package main

import "fmt"

const (
	Difficulty  = 3
	BlockReward = 10.0
)

type Blockchain struct {
	Blocks []*Block
}

func NewBlockchain(minerAddress string) *Blockchain {
	genesis := NewBlock(0, []*Transaction{NewCoinbaseTx(minerAddress, BlockReward)}, "0")
	genesis.Mine(Difficulty)

	return &Blockchain{Blocks: []*Block{genesis}}
}

func (bc *Blockchain) AddBlock(txs []*Transaction, minerAddress string) *Block {
	coinbase := NewCoinbaseTx(minerAddress, BlockReward)
	txs = append([]*Transaction{coinbase}, txs...)

	prev := bc.Blocks[len(bc.Blocks)-1]
	block := NewBlock(prev.Index+1, txs, prev.Hash)
	block.Mine(Difficulty)

	bc.Blocks = append(bc.Blocks, block)
	return block
}

func (bc *Blockchain) IsValid() bool {
	for i := 1; i < len(bc.Blocks); i++ {
		prev := bc.Blocks[i-1]
		curr := bc.Blocks[i]
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

	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			for i, out := range tx.Outputs {
				if out.Address != address {
					continue
				}
				if !spent[tx.ID][i] {
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

	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			for i, out := range tx.Outputs {
				if out.Address != address || spent[tx.ID][i] {
					continue
				}
				unspent[tx.ID] = append(unspent[tx.ID], i)
				total += out.Amount
				if total >= amount {
					return unspent, total
				}
			}
		}
	}
	return unspent, total
}

func (bc *Blockchain) spentOutputs(address string) map[string]map[int]bool {
	spent := make(map[string]map[int]bool)

	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.IsCoinbase() {
				continue
			}
			for _, in := range tx.Inputs {
				if len(in.PubKey) >= 64 {
					addr := pubKeyToAddress(in.PubKey)
					if addr == address {
						if spent[in.TxID] == nil {
							spent[in.TxID] = make(map[int]bool)
						}
						spent[in.TxID][in.OutIndex] = true
					}
				}
			}
		}
	}
	return spent
}
