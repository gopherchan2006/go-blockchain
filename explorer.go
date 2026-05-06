package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

//go:embed explorer.html
var explorerHTML []byte

type blockDTO struct {
	Index        int       `json:"index"`
	Hash         string    `json:"hash"`
	PrevHash     string    `json:"prevHash"`
	Nonce        int       `json:"nonce"`
	Timestamp    time.Time `json:"timestamp"`
	Transactions []txDTO   `json:"transactions"`
}

type txDTO struct {
	ID      string      `json:"id"`
	Type    string      `json:"type"`
	Data    string      `json:"data,omitempty"`
	Inputs  []inputDTO  `json:"inputs"`
	Outputs []outputDTO `json:"outputs"`
}

type inputDTO struct {
	TxID     string `json:"txId"`
	OutIndex int    `json:"outIndex"`
}

type outputDTO struct {
	Address string  `json:"address"`
	Amount  float64 `json:"amount"`
}

type walletDTO struct {
	Name    string  `json:"name"`
	Address string  `json:"address"`
	Balance float64 `json:"balance"`
}

type infoDTO struct {
	Height      int     `json:"height"`
	Valid       bool    `json:"valid"`
	Difficulty  int     `json:"difficulty"`
	BlockReward float64 `json:"blockReward"`
}

func blockToDTO(b *Block) blockDTO {
	dto := blockDTO{
		Index:     b.Index,
		Hash:      b.Hash,
		PrevHash:  b.PrevHash,
		Nonce:     b.Nonce,
		Timestamp: time.Unix(0, b.Timestamp),
	}
	for _, tx := range b.Transactions {
		txType := "transfer"
		if tx.IsCoinbase() {
			txType = "coinbase"
		} else if tx.Data != "" {
			txType = "data"
		}
		t := txDTO{ID: tx.ID, Type: txType, Data: tx.Data}
		for _, in := range tx.Inputs {
			t.Inputs = append(t.Inputs, inputDTO{TxID: in.TxID, OutIndex: in.OutIndex})
		}
		for _, out := range tx.Outputs {
			t.Outputs = append(t.Outputs, outputDTO{Address: out.Address, Amount: out.Amount})
		}
		dto.Transactions = append(dto.Transactions, t)
	}
	return dto
}

func RunExplorer(bcPath, walletsPath string, port int) error {
	bc, err := OpenBlockchain(bcPath)
	if err != nil {
		return fmt.Errorf("cannot open blockchain at %s: %w", bcPath, err)
	}
	defer bc.Close()

	wm, err := NewWalletManager(walletsPath)
	if err != nil {
		return fmt.Errorf("cannot open wallets at %s: %w", walletsPath, err)
	}
	defer wm.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(infoDTO{
			Height:      bc.Height(),
			Valid:       bc.IsValid(),
			Difficulty:  Difficulty,
			BlockReward: BlockReward,
		})
	})

	mux.HandleFunc("/api/blocks", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		var blocks []blockDTO
		for i := 0; i <= bc.Height(); i++ {
			b, err := bc.getBlock(i)
			if err != nil {
				continue
			}
			blocks = append(blocks, blockToDTO(b))
		}
		json.NewEncoder(w).Encode(blocks)
	})

	mux.HandleFunc("/api/wallets", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		names, _ := wm.ListWallets()
		var wallets []walletDTO
		for _, name := range names {
			addr, err := wm.GetAddress(name)
			if err != nil || addr == "" {
				continue
			}
			wallets = append(wallets, walletDTO{
				Name:    name,
				Address: addr,
				Balance: bc.GetBalance(addr),
			})
		}
		json.NewEncoder(w).Encode(wallets)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(explorerHTML)
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("  Explorer: http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}
