package main

import (
	"log"
)

func main() {
	if err := RunNode("./blockchain.db", "./wallets.db", 3030); err != nil {
		log.Fatal(err)
	}
}
