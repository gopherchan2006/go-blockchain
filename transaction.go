package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

type TxInput struct {
	TxID      string
	OutIndex  int
	Signature []byte
	PubKey    []byte
}

type TxOutput struct {
	Amount  float64
	Address string
}

type Transaction struct {
	ID      string
	Inputs  []TxInput
	Outputs []TxOutput
}

func (tx *Transaction) IsCoinbase() bool {
	return len(tx.Inputs) == 1 && tx.Inputs[0].TxID == "" && tx.Inputs[0].OutIndex == -1
}

func (tx *Transaction) Hash() string {
	data := fmt.Sprintf("%v%v", tx.Inputs, tx.Outputs)
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

func (tx *Transaction) Sign(privKey *ecdsa.PrivateKey) error {
	if tx.IsCoinbase() {
		return nil
	}
	dataToSign := tx.dataToSign()
	h := sha256.Sum256([]byte(dataToSign))

	r, s, err := ecdsa.Sign(rand.Reader, privKey, h[:])
	if err != nil {
		return err
	}

	sig := append(r.Bytes(), s.Bytes()...)
	pubKeyBytes := append(privKey.PublicKey.X.Bytes(), privKey.PublicKey.Y.Bytes()...)

	for i := range tx.Inputs {
		tx.Inputs[i].Signature = sig
		tx.Inputs[i].PubKey = pubKeyBytes
	}
	return nil
}

func (tx *Transaction) Verify() bool {
	if tx.IsCoinbase() {
		return true
	}

	dataToSign := tx.dataToSign()
	h := sha256.Sum256([]byte(dataToSign))

	for _, input := range tx.Inputs {
		if len(input.PubKey) < 64 {
			return false
		}

		x := new(big.Int).SetBytes(input.PubKey[:32])
		y := new(big.Int).SetBytes(input.PubKey[32:])
		pubKey := &ecdsa.PublicKey{Curve: ellipticCurve(), X: x, Y: y}

		if len(input.Signature) < 64 {
			return false
		}
		r := new(big.Int).SetBytes(input.Signature[:32])
		s := new(big.Int).SetBytes(input.Signature[32:])

		if !ecdsa.Verify(pubKey, h[:], r, s) {
			return false
		}
	}
	return true
}

func (tx *Transaction) dataToSign() string {
	var data string
	for _, in := range tx.Inputs {
		data += fmt.Sprintf("%s%d", in.TxID, in.OutIndex)
	}
	for _, out := range tx.Outputs {
		data += fmt.Sprintf("%s%.8f", out.Address, out.Amount)
	}
	return data
}

func NewCoinbaseTx(toAddress string, reward float64) *Transaction {
	input := TxInput{TxID: "", OutIndex: -1}
	output := TxOutput{Amount: reward, Address: toAddress}

	tx := &Transaction{
		Inputs:  []TxInput{input},
		Outputs: []TxOutput{output},
	}
	tx.ID = tx.Hash()
	return tx
}

func NewTransaction(from *Wallet, toAddress string, amount float64, bc *Blockchain) (*Transaction, error) {
	utxos, total := bc.FindSpendableUTXOs(from.Address(), amount)

	if total < amount {
		return nil, fmt.Errorf("insufficient funds: need %.2f, available %.2f", amount, total)
	}

	var inputs []TxInput
	for txID, outIndexes := range utxos {
		for _, idx := range outIndexes {
			inputs = append(inputs, TxInput{
				TxID:     txID,
				OutIndex: idx,
				PubKey:   append(from.PrivKey.PublicKey.X.Bytes(), from.PrivKey.PublicKey.Y.Bytes()...),
			})
		}
	}

	outputs := []TxOutput{
		{Amount: amount, Address: toAddress},
	}
	// сдача обратно отправителю
	if total > amount {
		outputs = append(outputs, TxOutput{Amount: total - amount, Address: from.Address()})
	}

	tx := &Transaction{Inputs: inputs, Outputs: outputs}
	tx.ID = tx.Hash()

	if err := tx.Sign(from.PrivKey); err != nil {
		return nil, err
	}
	tx.ID = tx.Hash()
	return tx, nil
}
