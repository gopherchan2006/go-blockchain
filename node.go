package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
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

	const mempoolPath = "./mempool.dat"
	if err := node.mempool.Load(mempoolPath); err != nil {
		fmt.Printf("  Warning: could not load mempool: %v\n", err)
	} else if node.mempool.Size() > 0 {
		fmt.Printf("  Loaded %d pending transactions from mempool.dat\n", node.mempool.Size())
	}

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

	mux.HandleFunc("/api/wallet/export", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Name     string `json:"name"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		wallet, err := node.wm.LoadWallet(req.Name, req.Password)
		if err != nil {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		pad32 := func(b []byte) []byte {
			if len(b) >= 32 {
				return b
			}
			out := make([]byte, 32)
			copy(out[32-len(b):], b)
			return out
		}
		d := pad32(wallet.PrivKey.D.Bytes())
		x := pad32(wallet.PrivKey.PublicKey.X.Bytes())
		y := pad32(wallet.PrivKey.PublicKey.Y.Bytes())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(struct {
			Address    string `json:"address"`
			PrivateKey string `json:"privateKey"`
			PublicKey  string `json:"publicKey"`
		}{
			Address:    wallet.Address(),
			PrivateKey: fmt.Sprintf("%x", d),
			PublicKey:  fmt.Sprintf("%x%x", x, y),
		})
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
		tx.ID = tx.Hash()
		if err := node.mempool.Add(&tx); err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
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

	srv := &http.Server{Addr: addr, Handler: mux}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-quit
		fmt.Println("\n  Shutting down...")
		if err := node.mempool.Save(mempoolPath); err != nil {
			fmt.Printf("  Warning: could not save mempool: %v\n", err)
		} else {
			fmt.Printf("  Saved %d pending transactions to mempool.dat\n", node.mempool.Size())
		}
		srv.Shutdown(context.Background())
	}()

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}
