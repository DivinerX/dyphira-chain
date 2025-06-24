package main

import (
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	ecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/sha3"
)

func setupTxPoolTest() (*State, *btcec.PrivateKey, Address, Address) {
	state := NewState()
	priv, _ := btcec.NewPrivateKey()
	addr1 := pubKeyToAddress(priv.PubKey())
	addr2 := Address{2}
	acc1 := &Account{Address: addr1, Balance: 100, Nonce: 0}
	acc2 := &Account{Address: addr2, Balance: 50, Nonce: 0}
	_ = state.PutAccount(acc1)
	_ = state.PutAccount(acc2)
	return state, priv, addr1, addr2
}

func makePoolTestTx(from, to Address, value, nonce uint64, priv *btcec.PrivateKey) *Transaction {
	tx := &Transaction{
		From:  from,
		To:    to,
		Value: value,
		Nonce: nonce,
		Fee:   0,
	}
	data, _ := tx.Encode()
	tx.Hash = sha3.Sum256(data)
	sig := ecdsa.Sign(priv, tx.Hash[:])
	tx.Signature = sig.Serialize()
	return tx
}

func TestTransactionPool(t *testing.T) {
	state, priv, addr1, addr2 := setupTxPoolTest()
	tx1 := makePoolTestTx(addr1, addr2, 10, 1, priv)
	tp := NewTransactionPool()

	// Add transaction
	err := tp.AddTransaction(tx1, priv.PubKey(), state)
	assert.Nil(t, err)
	assert.Equal(t, 1, tp.Size())

	// Duplicate add
	err = tp.AddTransaction(tx1, priv.PubKey(), state)
	assert.NotNil(t, err)
	assert.Equal(t, 1, tp.Size())

	// Remove transaction
	tp.RemoveTransaction(tx1.Hash)
	assert.Equal(t, 0, tp.Size())

	// Add multiple and select - with a new pool and state to ensure no leakage.
	tp2 := NewTransactionPool()
	state2, priv2, addr1_2, addr2_2 := setupTxPoolTest()
	tx2_1 := makePoolTestTx(addr1_2, addr2_2, 10, 1, priv2)
	tx2_2 := makePoolTestTx(addr1_2, addr2_2, 5, 2, priv2)
	tx2_3 := makePoolTestTx(addr1_2, addr2_2, 2, 3, priv2)

	// Add valid txs
	err = tp2.AddTransaction(tx2_1, priv2.PubKey(), state2)
	assert.Nil(t, err)

	// Manually update state for next valid tx
	acc1_2, _ := state2.GetAccount(addr1_2)
	acc1_2.Nonce++
	state2.PutAccount(acc1_2)

	err = tp2.AddTransaction(tx2_2, priv2.PubKey(), state2)
	assert.Nil(t, err)

	// Manually update state for next valid tx
	acc1_2, _ = state2.GetAccount(addr1_2)
	acc1_2.Nonce++
	state2.PutAccount(acc1_2)

	err = tp2.AddTransaction(tx2_3, priv2.PubKey(), state2)
	assert.Nil(t, err)

	// The state's nonce for the account is now 2.
	// The current implementation of SelectTransactions should only find
	// transactions with nonce 3 (state.Nonce + 1).
	sel := tp2.SelectTransactions(10, state2)
	assert.Equal(t, 1, len(sel), "Should only select the single next valid transaction")
	assert.Equal(t, tx2_3.Hash, sel[0].Hash)
}

func TestParticipationTransaction(t *testing.T) {
	state, priv, addr1, _ := setupTxPoolTest()
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
	sig := ecdsa.Sign(priv, tx.Hash[:])
	tx.Signature = sig.Serialize()

	err := tp.AddTransaction(tx, priv.PubKey(), state)
	assert.Nil(t, err)

	block := &Block{Transactions: []*Transaction{tx}}
	bc, _ := NewBlockchain(NewMemoryStore())
	state2 := NewState()
	_ = state2.PutAccount(&Account{Address: addr1, Balance: 100, Nonce: 0})
	_ = bc.ApplyBlockWithRegistry(block, state2, vr)

	v2, _ := vr.GetValidator(addr1)
	assert.True(t, v2.Participating, "Validator should be marked as participating after participation tx")
}
