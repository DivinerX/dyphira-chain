package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/sha3"
)

var testKey, _ = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
var testAddr = pubKeyToAddress(&testKey.PublicKey)

func makeTestTx(to Address, value, nonce uint64) *Transaction {
	tx := &Transaction{
		From:  testAddr,
		To:    to,
		Value: value,
		Nonce: nonce,
	}
	data, _ := tx.Encode()
	tx.Hash = sha3.Sum256(data)
	r, s, _ := ecdsa.Sign(rand.Reader, testKey, tx.Hash[:])
	tx.R = r.Bytes()
	tx.S = s.Bytes()
	return tx
}

func TestApplyTransaction(t *testing.T) {
	s := NewState()
	fromAccount := &Account{Address: testAddr, Balance: 100, Nonce: 0}
	s.PutAccount(fromAccount)

	tx := makeTestTx(Address{1}, 10, 1)

	err := s.ApplyTransaction(tx)
	assert.Nil(t, err)

	fromAccount, _ = s.GetAccount(testAddr)
	assert.Equal(t, uint64(90), fromAccount.Balance)
	assert.Equal(t, uint64(1), fromAccount.Nonce)

	toAccount, _ := s.GetAccount(Address{1})
	assert.Equal(t, uint64(10), toAccount.Balance)

	// Test invalid nonce
	tx = makeTestTx(Address{1}, 10, 3)
	err = s.ApplyTransaction(tx)
	assert.NotNil(t, err)
}

func TestState(t *testing.T) {
	s := NewState()
	addr := Address{1}
	acc := &Account{Address: addr, Balance: 100, Nonce: 1}

	err := s.PutAccount(acc)
	assert.Nil(t, err)

	retrieved, err := s.GetAccount(addr)
	assert.Nil(t, err)
	assert.Equal(t, acc, retrieved)
}

func TestState_ApplyBlock(t *testing.T) {
	s := NewState()
	addr1 := Address{1}
	addr2 := Address{2}
	acc1 := &Account{Address: addr1, Balance: 100, Nonce: 0}
	acc2 := &Account{Address: addr2, Balance: 50, Nonce: 0}
	s.PutAccount(acc1)
	s.PutAccount(acc2)

	tx1 := &Transaction{From: addr1, To: addr2, Value: 10, Nonce: 1}
	tx1Data, _ := tx1.Encode()
	tx1.Hash = sha3.Sum256(tx1Data)

	tx2 := &Transaction{From: addr1, To: addr2, Value: 5, Nonce: 2}
	tx2Data, _ := tx2.Encode()
	tx2.Hash = sha3.Sum256(tx2Data)

	txs := []*Transaction{tx1, tx2}
	block := &Block{Transactions: txs}

	err := s.ApplyBlock(block)
	assert.Nil(t, err)

	acc1, _ = s.GetAccount(addr1)
	assert.Equal(t, uint64(85), acc1.Balance)
	assert.Equal(t, uint64(2), acc1.Nonce)

	acc2, _ = s.GetAccount(addr2)
	assert.Equal(t, uint64(65), acc2.Balance)
}
