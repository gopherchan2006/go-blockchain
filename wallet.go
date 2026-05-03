package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"

	"golang.org/x/crypto/ripemd160"
)

func ellipticCurve() elliptic.Curve {
	return elliptic.P256()
}

type Wallet struct {
	PrivKey *ecdsa.PrivateKey
}

func NewWallet() (*Wallet, error) {
	privKey, err := ecdsa.GenerateKey(ellipticCurve(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &Wallet{PrivKey: privKey}, nil
}

func (w *Wallet) Address() string {
	pubKey := append(w.PrivKey.PublicKey.X.Bytes(), w.PrivKey.PublicKey.Y.Bytes()...)
	sha := sha256.Sum256(pubKey)
	ripe := ripemd160.New()
	ripe.Write(sha[:])
	hash := ripe.Sum(nil)
	return hex.EncodeToString(hash)
}

func (w *Wallet) PublicKeyBytes() []byte {
	return append(w.PrivKey.PublicKey.X.Bytes(), w.PrivKey.PublicKey.Y.Bytes()...)
}

func pubKeyToAddress(pubKey []byte) string {
	sha := sha256.Sum256(pubKey)
	ripe := ripemd160.New()
	ripe.Write(sha[:])
	return hex.EncodeToString(ripe.Sum(nil))
}
