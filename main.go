package main

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	port := envInt("PORT", 3030)
	p2pPort := envInt("P2P_PORT", port+1000)
	dataDir := envString("DATA_DIR", ".")
	peers := envCSV("PEERS")
	bcPath := filepath.Join(dataDir, "blockchain.db")
	walletsPath := filepath.Join(dataDir, "wallets.db")
	if err := RunNode(bcPath, walletsPath, port, p2pPort, peers); err != nil {
		log.Fatal(err)
	}
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envString(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func envCSV(key string) []string {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
