package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/sha3"
)

var testState *State

func setupTxPoolTest() (*ecdsa.PrivateKey, Address, Address) {
	testState = NewState()
	priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	addr1 := pubKeyToAddress(&priv.PublicKey)
	addr2 := Address{2}
	acc1 := &Account{Address: addr1, Balance: 100, Nonce: 0}
	acc2 := &Account{Address: addr2, Balance: 50, Nonce: 0}
	_ = testState.PutAccount(acc1)
	_ = testState.PutAccount(acc2)
	return priv, addr1, addr2
}

func makePoolTestTx(from, to Address, value, nonce uint64, priv *ecdsa.PrivateKey) *Transaction {
	tx := &Transaction{
		From:  from,
		To:    to,
		Value: value,
		Nonce: nonce,
		Fee:   0,
	}
	data, _ := tx.Encode()
	tx.Hash = sha3.Sum256(data)
	r, s, _ := ecdsa.Sign(rand.Reader, priv, tx.Hash[:])
	tx.R = r.Bytes()
	tx.S = s.Bytes()
	return tx
}

func TestTransactionPool(t *testing.T) {
	priv, addr1, addr2 := setupTxPoolTest()
	tx1 := makePoolTestTx(addr1, addr2, 10, 1, priv)
	tp := NewTransactionPool()

	// Add transaction
	err := tp.AddTransaction(tx1, &priv.PublicKey, testState)
	assert.Nil(t, err)
	assert.Equal(t, 1, tp.Size())

	// Duplicate add
	err = tp.AddTransaction(tx1, &priv.PublicKey, testState)
	assert.NotNil(t, err)
	assert.Equal(t, 1, tp.Size())

	// Remove transaction
	tp.RemoveTransaction(tx1.Hash)
	assert.Equal(t, 0, tp.Size())

	// Add multiple and select
	priv, addr1, addr2 = setupTxPoolTest() // Reset state and get new addresses
	tx1 = makePoolTestTx(addr1, addr2, 10, 1, priv)
	tx2 := makePoolTestTx(addr1, addr2, 5, 2, priv)
	tx3 := makePoolTestTx(addr1, addr2, 2, 3, priv)

	// Add valid txs
	err = tp.AddTransaction(tx1, &priv.PublicKey, testState)
	assert.Nil(t, err)

	// Manually update state for next valid tx
	acc1, _ := testState.GetAccount(addr1)
	acc1.Nonce++
	testState.PutAccount(acc1)

	err = tp.AddTransaction(tx2, &priv.PublicKey, testState)
	assert.Nil(t, err)

	// Manually update state for next valid tx
	acc1, _ = testState.GetAccount(addr1)
	acc1.Nonce++
	testState.PutAccount(acc1)

	err = tp.AddTransaction(tx3, &priv.PublicKey, testState)
	assert.Nil(t, err)

	sel := tp.SelectTransactions(2)
	assert.Equal(t, 2, len(sel))

	sel = tp.SelectTransactions(10)
	assert.Equal(t, 3, len(sel))
}

func TestParticipationTransaction(t *testing.T) {
	priv, addr1, _ := setupTxPoolTest()
	vr := NewValidatorRegistry(NewMemoryStore(), "validators")
	v := &Validator{Address: addr1, Stake: 100}
	_ = vr.RegisterValidator(v)

	tp := NewTransactionPool()

	// Create a participation transaction
	tx := &Transaction{
		From:  addr1,
		To:    addr1,
		Value: 0,
		Nonce: 1,
		Type:  "participation",
	}
	data, _ := tx.Encode()
	tx.Hash = sha3.Sum256(data)
	r, s, _ := ecdsa.Sign(rand.Reader, priv, tx.Hash[:])
	tx.R = r.Bytes()
	tx.S = s.Bytes()

	err := tp.AddTransaction(tx, &priv.PublicKey, testState)
	assert.Nil(t, err)

	block := &Block{Transactions: []*Transaction{tx}}
	bc, _ := NewBlockchain(NewMemoryStore())
	state := NewState()
	_ = state.PutAccount(&Account{Address: addr1, Balance: 100, Nonce: 0})
	_ = bc.ApplyBlockWithRegistry(block, state, vr)

	v2, _ := vr.GetValidator(addr1)
	assert.True(t, v2.Participating, "Validator should be marked as participating after participation tx")
}
