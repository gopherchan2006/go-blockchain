package main

import (
	"fmt"
	"log"
)

func main() {
	fmt.Println("=== Blockchain Demo ===")

	fmt.Println("\n► Creating wallets...")
	alice, err := NewWallet()
	if err != nil {
		log.Fatal(err)
	}
	bob, err := NewWallet()
	if err != nil {
		log.Fatal(err)
	}
	miner, err := NewWallet()
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("  Alice: %s\n", alice.Address())
	fmt.Printf("  Bob:   %s\n", bob.Address())
	fmt.Printf("  Miner: %s\n\n", miner.Address())

	fmt.Println("► Mining genesis block (reward → Alice)...")
	bc, err := NewBlockchain("./blockchain.db", alice.Address())
	if err != nil {
		log.Fatal(err)
	}
	defer bc.Close()

	printBalances(bc, alice, bob, miner)
	SaveAsJSON(bc, "blockchain.json")

	fmt.Println("► Alice sends Bob 3 coins...")
	tx1, err := NewTransaction(alice, bob.Address(), 3.0, bc)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("► Mining block #1...")
	_, err = bc.AddBlock([]*Transaction{tx1}, miner.Address())
	if err != nil {
		log.Fatal(err)
	}
	printBalances(bc, alice, bob, miner)

	fmt.Println("► Bob sends Alice 1 coin...")
	tx2, err := NewTransaction(bob, alice.Address(), 1.0, bc)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println("► Mining block #2...")
	_, err = bc.AddBlock([]*Transaction{tx2}, miner.Address())
	if err != nil {
		log.Fatal(err)
	}
	printBalances(bc, alice, bob, miner)

	fmt.Println("► Validating chain...")
	if bc.IsValid() {
		fmt.Println("  ✓ Chain is valid!\n")
	}

	fmt.Println("► Storing arbitrary data on-chain (like Bible verse in BTC block 666,666)...")
	dataMessage := "Do not be overcome by evil, but overcome evil with good — Romans 12:21"
	tx3, err := NewDataTransaction(alice, dataMessage, bc)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("  Message: \"%s\"\n", dataMessage)
	fmt.Printf("  Message hash: %s\n\n", tx3.Hash())

	fmt.Println("► Mining block #3 with embedded data...")
	block, err := bc.AddBlock([]*Transaction{tx3}, miner.Address())
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("  Block data: %s\n", block.Transactions[1].Data)
	fmt.Printf("  This data is now permanently in the blockchain!\n\n")

	fmt.Println("=== Done ===")
}

func printBalances(bc *Blockchain, alice, bob, miner *Wallet) {
	fmt.Printf("  Balances → Alice: %.1f | Bob: %.1f | Miner: %.1f\n\n",
		bc.GetBalance(alice.Address()),
		bc.GetBalance(bob.Address()),
		bc.GetBalance(miner.Address()),
	)
}
