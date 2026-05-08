package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/gob"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
	"golang.org/x/crypto/pbkdf2"
)

type WalletManager struct {
	db *leveldb.DB
}

type walletRecord struct {
	Address    string
	Salt       []byte
	Nonce      []byte
	Ciphertext []byte
}

type storedKey struct {
	D []byte
	X []byte
	Y []byte
}

type WalletMeta struct {
	Name    string
	Address string
}

func NewWalletManager(dbPath string) (*WalletManager, error) {
	db, err := leveldb.OpenFile(dbPath, nil)
	if err != nil {
		return nil, err
	}
	return &WalletManager{db: db}, nil
}

func (wm *WalletManager) Close() error {
	return wm.db.Close()
}

func deriveKey(password string, salt []byte) []byte {
	return pbkdf2.Key([]byte(password), salt, 100000, 32, sha256.New)
}

func (wm *WalletManager) CreateWallet(name, password string) (*Wallet, error) {
	key := []byte("wallet:" + name)
	exists, err := wm.db.Has(key, nil)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.New("wallet already exists: " + name)
	}

	wallet, err := NewWallet()
	if err != nil {
		return nil, err
	}

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}

	aesKey := deriveKey(password, salt)
	sk := storedKey{
		D: wallet.PrivKey.D.Bytes(),
		X: wallet.PrivKey.PublicKey.X.Bytes(),
		Y: wallet.PrivKey.PublicKey.Y.Bytes(),
	}
	var plainBuf bytes.Buffer
	if err := gob.NewEncoder(&plainBuf).Encode(sk); err != nil {
		return nil, err
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nil, nonce, plainBuf.Bytes(), nil)

	record := walletRecord{Address: wallet.Address(), Salt: salt, Nonce: nonce, Ciphertext: ciphertext}
	var recBuf bytes.Buffer
	if err := gob.NewEncoder(&recBuf).Encode(record); err != nil {
		return nil, err
	}
	return wallet, wm.db.Put(key, recBuf.Bytes(), nil)
}

func (wm *WalletManager) LoadWallet(name, password string) (*Wallet, error) {
	key := []byte("wallet:" + name)
	data, err := wm.db.Get(key, nil)
	if err != nil {
		return nil, errors.New("wallet not found: " + name)
	}

	var record walletRecord
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&record); err != nil {
		return nil, err
	}

	aesKey := deriveKey(password, record.Salt)
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err := gcm.Open(nil, record.Nonce, record.Ciphertext, nil)
	if err != nil {
		return nil, errors.New("wrong password or corrupted wallet")
	}

	var sk storedKey
	if err := gob.NewDecoder(bytes.NewReader(plaintext)).Decode(&sk); err != nil {
		return nil, err
	}

	privKey := &ecdsa.PrivateKey{
		PublicKey: ecdsa.PublicKey{
			Curve: ellipticCurve(),
			X:     new(big.Int).SetBytes(sk.X),
			Y:     new(big.Int).SetBytes(sk.Y),
		},
		D: new(big.Int).SetBytes(sk.D),
	}
	return &Wallet{PrivKey: privKey}, nil
}

func (wm *WalletManager) GetOrCreate(name, password string) (*Wallet, error) {
	w, err := wm.LoadWallet(name, password)
	if err != nil {
		return wm.CreateWallet(name, password)
	}
	return w, nil
}

func (wm *WalletManager) GetAddress(name string) (string, error) {
	key := []byte("wallet:" + name)
	data, err := wm.db.Get(key, nil)
	if err != nil {
		return "", errors.New("wallet not found: " + name)
	}
	var record walletRecord
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&record); err != nil {
		return "", err
	}
	return record.Address, nil
}

func (wm *WalletManager) ListWallets() ([]string, error) {
	iter := wm.db.NewIterator(util.BytesPrefix([]byte("wallet:")), nil)
	defer iter.Release()
	var names []string
	for iter.Next() {
		names = append(names, strings.TrimPrefix(string(iter.Key()), "wallet:"))
	}
	return names, iter.Error()
}

func (wm *WalletManager) ListWalletMetas() ([]WalletMeta, error) {
	names, err := wm.ListWallets()
	if err != nil {
		return nil, err
	}
	out := make([]WalletMeta, 0, len(names))
	for _, n := range names {
		addr, err := wm.GetAddress(n)
		if err != nil || addr == "" {
			continue
		}
		out = append(out, WalletMeta{Name: n, Address: addr})
	}
	return out, nil
}

func (wm *WalletManager) HasAddress(address string) (bool, error) {
	metas, err := wm.ListWalletMetas()
	if err != nil {
		return false, err
	}
	for _, m := range metas {
		if m.Address == address {
			return true, nil
		}
	}
	return false, nil
}

func (wm *WalletManager) CreateWatchWallet(name, address string) error {
	key := []byte("wallet:" + name)
	exists, err := wm.db.Has(key, nil)
	if err != nil {
		return err
	}
	if exists {
		current, err := wm.GetAddress(name)
		if err != nil {
			return err
		}
		if current == address {
			return nil
		}
		return fmt.Errorf("wallet already exists with different address: %s", name)
	}
	record := walletRecord{Address: address}
	var recBuf bytes.Buffer
	if err := gob.NewEncoder(&recBuf).Encode(record); err != nil {
		return err
	}
	return wm.db.Put(key, recBuf.Bytes(), nil)
}
