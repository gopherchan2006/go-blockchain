package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

type utxoDTO struct {
	TxID     string  `json:"txID"`
	OutIndex int     `json:"outIndex"`
	Amount   float64 `json:"amount"`
}

type blockTemplateDTO struct {
	Index      int    `json:"index"`
	Timestamp  int64  `json:"timestamp"`
	TxData     string `json:"txData"`
	PrevHash   string `json:"prevHash"`
	Difficulty int    `json:"difficulty"`
}

type submitBlockDTO struct {
	Nonce int    `json:"nonce"`
	Hash  string `json:"hash"`
}

type Node struct {
	bc       *Blockchain
	wm       *WalletManager
	mempool  *Mempool
	hub      *EventHub
	mu       sync.Mutex
	template *Block
}

func NewNode(bcPath, walletsPath string) (*Node, error) {
	bc, err := OpenBlockchain(bcPath)
	if err != nil {
		return nil, fmt.Errorf("cannot open blockchain: %w", err)
	}
	wm, err := NewWalletManager(walletsPath)
	if err != nil {
		bc.Close()
		return nil, fmt.Errorf("cannot open wallets: %w", err)
	}
	return &Node{
		bc:      bc,
		wm:      wm,
		mempool: NewMempool(),
		hub:     NewEventHub(),
	}, nil
}

func (n *Node) refreshTemplate(minerAddress string) {
	txs := n.mempool.Peek()
	tmpl, err := n.bc.NewBlockTemplate(minerAddress, txs)
	if err == nil {
		n.template = tmpl
	}
}

func RunNode(bcPath, walletsPath string, port int) error {
	node, err := NewNode(bcPath, walletsPath)
	if err != nil {
		return err
	}
	defer node.bc.Close()
	defer node.wm.Close()

	mux := http.NewServeMux()

	mux.HandleFunc("/api/info", func(w http.ResponseWriter, r *http.Request) {
		node.mu.Lock()
		defer node.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(infoDTO{
			Height:      node.bc.Height(),
			Valid:       node.bc.IsValid(),
			Difficulty:  Difficulty,
			BlockReward: BlockReward,
		})
	})

	mux.HandleFunc("/api/blocks", func(w http.ResponseWriter, r *http.Request) {
		node.mu.Lock()
		defer node.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		var blocks []blockDTO
		for i := 0; i <= node.bc.Height(); i++ {
			b, err := node.bc.getBlock(i)
			if err != nil {
				continue
			}
			dto := blockToDTO(b)
			blocks = append(blocks, dto)
		}
		json.NewEncoder(w).Encode(blocks)
	})

	mux.HandleFunc("/api/wallets", func(w http.ResponseWriter, r *http.Request) {
		node.mu.Lock()
		defer node.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		names, _ := node.wm.ListWallets()
		var wallets []walletDTO
		for _, name := range names {
			addr, err := node.wm.GetAddress(name)
			if err != nil || addr == "" {
				continue
			}
			wallets = append(wallets, walletDTO{
				Name:    name,
				Address: addr,
				Balance: node.bc.GetBalance(addr),
			})
		}
		json.NewEncoder(w).Encode(wallets)
	})

	mux.HandleFunc("/api/mempool", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(node.mempool.Peek())
	})

	mux.HandleFunc("/api/utxos", func(w http.ResponseWriter, r *http.Request) {
		address := r.URL.Query().Get("address")
		if address == "" {
			http.Error(w, "address required", http.StatusBadRequest)
			return
		}
		node.mu.Lock()
		defer node.mu.Unlock()
		spent := node.bc.spentOutputs(address)
		var utxos []utxoDTO
		for i := 0; i <= node.bc.Height(); i++ {
			block, err := node.bc.getBlock(i)
			if err != nil {
				continue
			}
			for _, tx := range block.Transactions {
				for j, out := range tx.Outputs {
					if out.Address == address && !spent[tx.ID][j] {
						utxos = append(utxos, utxoDTO{TxID: tx.ID, OutIndex: j, Amount: out.Amount})
					}
				}
			}
		}
		if utxos == nil {
			utxos = []utxoDTO{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(utxos)
	})

	mux.HandleFunc("/api/transaction", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var tx Transaction
		if err := json.NewDecoder(r.Body).Decode(&tx); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if !tx.Verify() {
			http.Error(w, "invalid transaction signature", http.StatusBadRequest)
			return
		}
		node.mempool.Add(&tx)
		node.hub.Broadcast("new_tx", tx.ID)
		w.WriteHeader(http.StatusAccepted)
	})

	mux.HandleFunc("/api/blocktemplate", func(w http.ResponseWriter, r *http.Request) {
		minerAddr := r.URL.Query().Get("miner")
		if minerAddr == "" {
			http.Error(w, "miner address required", http.StatusBadRequest)
			return
		}
		node.mu.Lock()
		node.refreshTemplate(minerAddr)
		tmpl := node.template
		node.mu.Unlock()

		if tmpl == nil {
			http.Error(w, "template not ready", http.StatusServiceUnavailable)
			return
		}

		txDataBytes, err := json.Marshal(tmpl.Transactions)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blockTemplateDTO{
			Index:      tmpl.Index,
			Timestamp:  tmpl.Timestamp,
			TxData:     string(txDataBytes),
			PrevHash:   tmpl.PrevHash,
			Difficulty: Difficulty,
		})
	})

	mux.HandleFunc("/api/block", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var sub submitBlockDTO
		if err := json.NewDecoder(r.Body).Decode(&sub); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}

		node.mu.Lock()
		defer node.mu.Unlock()

		if node.template == nil {
			http.Error(w, "no active template", http.StatusConflict)
			return
		}

		node.template.Nonce = sub.Nonce
		node.template.Hash = sub.Hash

		if err := node.bc.SubmitBlock(node.template); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		node.mempool.Flush()
		node.template = nil
		node.hub.Broadcast("block_mined", fmt.Sprintf("%d", node.bc.Height()))
		w.WriteHeader(http.StatusCreated)
	})

	mux.HandleFunc("/api/events", node.hub.ServeHTTP)

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write(explorerHTML)
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("  Node: http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}
