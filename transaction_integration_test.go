package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTransactionIntegration tests the complete transaction flow from creation to finalization
func TestTransactionIntegration(t *testing.T) {
	// Setup
	store := NewMemoryStore()
	bc, err := NewBlockchain(store)
	require.NoError(t, err)

	state := NewState()
	txPool := NewTransactionPool()
	vr := NewValidatorRegistry(NewMemoryStore(), "validators")

	// Create test accounts
	privKey1, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	privKey2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	addr1 := pubKeyToAddress(&privKey1.PublicKey)
	addr2 := pubKeyToAddress(&privKey2.PublicKey)

	// Fund accounts
	acc1 := &Account{Address: addr1, Balance: 1000, Nonce: 0}
	acc2 := &Account{Address: addr2, Balance: 500, Nonce: 0}
	require.NoError(t, state.PutAccount(acc1))
	require.NoError(t, state.PutAccount(acc2))

	// Register validators
	v1 := &Validator{Address: addr1, Stake: 100, Participating: true}
	v2 := &Validator{Address: addr2, Stake: 100, Participating: true}
	require.NoError(t, vr.RegisterValidator(v1))
	require.NoError(t, vr.RegisterValidator(v2))

	// Test 1: Create and validate transaction
	tx := &Transaction{
		From:      addr1,
		To:        addr2,
		Value:     100,
		Nonce:     1,
		Fee:       10,
		Type:      "transfer",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx.Sign(privKey1))

	// Test 2: Add transaction to pool
	err = txPool.AddTransaction(tx, &privKey1.PublicKey, state)
	assert.NoError(t, err)
	assert.Equal(t, 1, txPool.Size())

	// Test 3: Select transactions for block inclusion
	selectedTxs := txPool.SelectTransactions(10, state)
	assert.Len(t, selectedTxs, 1)
	assert.Equal(t, tx.Hash, selectedTxs[0].Hash)

	// Test 4: Create block with transaction
	proposer := v1
	block, err := bc.CreateBlock(selectedTxs, proposer, privKey1)
	require.NoError(t, err)
	assert.Len(t, block.Transactions, 1)
	assert.Equal(t, tx.Hash, block.Transactions[0].Hash)

	// Test 5: Apply block to blockchain
	err = bc.AddBlock(block)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), bc.Height())

	// Test 6: Apply block transactions to state
	err = bc.ApplyBlockWithRegistry(block, state, vr)
	assert.NoError(t, err)

	// Test 7: Remove transactions from pool (simulating finalizeApprovedBlock behavior)
	for _, tx := range block.Transactions {
		txPool.RemoveTransaction(tx.Hash)
	}

	// Test 8: Verify state changes
	updatedAcc1, err := state.GetAccount(addr1)
	assert.NoError(t, err)
	assert.Equal(t, uint64(890), updatedAcc1.Balance) // 1000 - 100 - 10
	assert.Equal(t, uint64(1), updatedAcc1.Nonce)

	updatedAcc2, err := state.GetAccount(addr2)
	assert.NoError(t, err)
	assert.Equal(t, uint64(600), updatedAcc2.Balance) // 500 + 100

	// Test 9: Verify transaction is removed from pool
	assert.Equal(t, 0, txPool.Size())
}

// TestTransactionPoolIntegration tests transaction pool integration with state
func TestTransactionPoolIntegration(t *testing.T) {
	state := NewState()
	txPool := NewTransactionPool()

	// Create test accounts
	privKey1, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	privKey2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	addr1 := pubKeyToAddress(&privKey1.PublicKey)
	addr2 := pubKeyToAddress(&privKey2.PublicKey)

	// Fund account 1 with more balance
	acc1 := &Account{Address: addr1, Balance: 200, Nonce: 0}
	require.NoError(t, state.PutAccount(acc1))

	// Test 1: Add first transaction (nonce 1) - should succeed
	tx1 := &Transaction{
		From:      addr1,
		To:        addr2,
		Value:     50,
		Nonce:     1,
		Fee:       5,
		Type:      "transfer",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx1.Sign(privKey1))
	err := txPool.AddTransaction(tx1, &privKey1.PublicKey, state)
	assert.NoError(t, err)
	assert.Equal(t, 1, txPool.Size())

	// Test 2: Try to add transaction with nonce 3 - should fail
	tx3 := &Transaction{
		From:      addr1,
		To:        addr2,
		Value:     20,
		Nonce:     3,
		Fee:       5,
		Type:      "transfer",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx3.Sign(privKey1))
	err = txPool.AddTransaction(tx3, &privKey1.PublicKey, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid nonce")
	assert.Equal(t, 1, txPool.Size()) // Pool size should not change

	// Test 3: Try to add second transaction (nonce 2) - should fail because account nonce is still 0
	tx2 := &Transaction{
		From:      addr1,
		To:        addr2,
		Value:     30,
		Nonce:     2,
		Fee:       5,
		Type:      "transfer",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx2.Sign(privKey1))
	err = txPool.AddTransaction(tx2, &privKey1.PublicKey, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid nonce")
	assert.Equal(t, 1, txPool.Size()) // Pool size should not change

	// Test 4: Select transactions - should only get nonce 1
	selectedTxs := txPool.SelectTransactions(10, state)
	assert.Len(t, selectedTxs, 1)
	assert.Equal(t, uint64(1), selectedTxs[0].Nonce)

	// Test 5: Apply first transaction and update state
	err = state.ApplyTransaction(selectedTxs[0])
	assert.NoError(t, err)
	txPool.MarkTransactionAsUsed(selectedTxs[0].Hash)

	// Test 6: Now nonce 2 should be acceptable
	err = txPool.AddTransaction(tx2, &privKey1.PublicKey, state)
	assert.NoError(t, err)
	// Note: Pool size might be 2 because the first transaction is still there but marked as used

	// Test 7: Now nonce 2 should be selectable
	selectedTxs = txPool.SelectTransactions(10, state)
	assert.Len(t, selectedTxs, 1)
	assert.Equal(t, uint64(2), selectedTxs[0].Nonce)

	// Test 8: Apply second transaction
	err = state.ApplyTransaction(selectedTxs[0])
	assert.NoError(t, err)
	txPool.MarkTransactionAsUsed(selectedTxs[0].Hash)

	// Test 9: Now nonce 3 should be acceptable
	err = txPool.AddTransaction(tx3, &privKey1.PublicKey, state)
	assert.NoError(t, err)

	// Test 10: Nonce 3 should be selectable
	selectedTxs = txPool.SelectTransactions(10, state)
	if assert.Len(t, selectedTxs, 1) {
		assert.Equal(t, uint64(3), selectedTxs[0].Nonce)
	}
}

// TestParticipationTransactionIntegration tests participation transaction flow
func TestParticipationTransactionIntegration(t *testing.T) {
	state := NewState()
	txPool := NewTransactionPool()
	vr := NewValidatorRegistry(NewMemoryStore(), "validators")

	// Create test validator
	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	addr := pubKeyToAddress(&privKey.PublicKey)

	// Register validator but not participating
	v := &Validator{Address: addr, Stake: 100, Participating: false}
	require.NoError(t, vr.RegisterValidator(v))

	// Create participation transaction
	tx := &Transaction{
		From:      addr,
		To:        addr,
		Value:     0,
		Nonce:     1,
		Fee:       0,
		Type:      "participation",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx.Sign(privKey))

	// Add to pool
	err := txPool.AddTransaction(tx, &privKey.PublicKey, state)
	assert.NoError(t, err)

	// Create and apply block
	bc, _ := NewBlockchain(NewMemoryStore())
	proposer := &Validator{Address: addr, Stake: 100}
	block, err := bc.CreateBlock([]*Transaction{tx}, proposer, privKey)
	require.NoError(t, err)

	err = bc.AddBlock(block)
	assert.NoError(t, err)

	err = bc.ApplyBlockWithRegistry(block, state, vr)
	assert.NoError(t, err)

	// Verify validator is now participating
	updatedV, err := vr.GetValidator(addr)
	assert.NoError(t, err)
	assert.True(t, updatedV.Participating)
}

// TestTransactionValidationIntegration tests transaction validation across components
func TestTransactionValidationIntegration(t *testing.T) {
	state := NewState()
	txPool := NewTransactionPool()

	// Create test accounts
	privKey1, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	privKey2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	addr1 := pubKeyToAddress(&privKey1.PublicKey)
	addr2 := pubKeyToAddress(&privKey2.PublicKey)

	// Fund account 1
	acc1 := &Account{Address: addr1, Balance: 100, Nonce: 0}
	require.NoError(t, state.PutAccount(acc1))

	// Test invalid nonce
	tx := &Transaction{
		From:      addr1,
		To:        addr2,
		Value:     50,
		Nonce:     3, // Should be 1
		Fee:       5,
		Type:      "transfer",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx.Sign(privKey1))

	err := txPool.AddTransaction(tx, &privKey1.PublicKey, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid nonce")

	// Test insufficient balance
	tx2 := &Transaction{
		From:      addr1,
		To:        addr2,
		Value:     200, // More than balance
		Nonce:     1,
		Fee:       5,
		Type:      "transfer",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx2.Sign(privKey1))

	err = txPool.AddTransaction(tx2, &privKey1.PublicKey, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient balance")

	// Test invalid signature
	tx3 := &Transaction{
		From:      addr1,
		To:        addr2,
		Value:     50,
		Nonce:     1,
		Fee:       5,
		Type:      "transfer",
		Timestamp: time.Now().UnixNano(),
	}
	// Sign with wrong private key
	require.NoError(t, tx3.Sign(privKey2))

	err = txPool.AddTransaction(tx3, &privKey1.PublicKey, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

// TestTransactionStateConsistency tests that transaction application maintains state consistency
func TestTransactionStateConsistency(t *testing.T) {
	state := NewState()
	bc, _ := NewBlockchain(NewMemoryStore())

	// Create test accounts
	privKey1, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	privKey2, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	addr1 := pubKeyToAddress(&privKey1.PublicKey)
	addr2 := pubKeyToAddress(&privKey2.PublicKey)

	// Fund accounts
	acc1 := &Account{Address: addr1, Balance: 1000, Nonce: 0}
	acc2 := &Account{Address: addr2, Balance: 500, Nonce: 0}
	require.NoError(t, state.PutAccount(acc1))
	require.NoError(t, state.PutAccount(acc2))

	// Create multiple transactions
	txs := []*Transaction{}
	for i := 1; i <= 3; i++ {
		tx := &Transaction{
			From:      addr1,
			To:        addr2,
			Value:     100,
			Nonce:     uint64(i),
			Fee:       10,
			Type:      "transfer",
			Timestamp: time.Now().UnixNano(),
		}
		require.NoError(t, tx.Sign(privKey1))
		txs = append(txs, tx)
	}

	// Create block with all transactions
	proposer := &Validator{Address: addr1, Stake: 100}
	block, err := bc.CreateBlock(txs, proposer, privKey1)
	require.NoError(t, err)

	// Apply block
	err = bc.AddBlock(block)
	assert.NoError(t, err)

	err = bc.ApplyBlockWithRegistry(block, state, nil)
	assert.NoError(t, err)

	// Verify final state
	updatedAcc1, err := state.GetAccount(addr1)
	assert.NoError(t, err)
	assert.Equal(t, uint64(670), updatedAcc1.Balance) // 1000 - (3 * 100) - (3 * 10)
	assert.Equal(t, uint64(3), updatedAcc1.Nonce)

	updatedAcc2, err := state.GetAccount(addr2)
	assert.NoError(t, err)
	assert.Equal(t, uint64(800), updatedAcc2.Balance) // 500 + (3 * 100)
}

// TestTransactionReplayProtection tests that transactions cannot be replayed
func TestTransactionReplayProtection(t *testing.T) {
	state := NewState()
	txPool := NewTransactionPool()

	// Create test accounts
	privKey, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	addr1 := pubKeyToAddress(&privKey.PublicKey)
	addr2 := Address{2}

	// Fund account
	acc1 := &Account{Address: addr1, Balance: 1000, Nonce: 0}
	require.NoError(t, state.PutAccount(acc1))

	// Create transaction
	tx := &Transaction{
		From:      addr1,
		To:        addr2,
		Value:     100,
		Nonce:     1,
		Fee:       10,
		Type:      "transfer",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx.Sign(privKey))

	// Add transaction to pool
	err := txPool.AddTransaction(tx, &privKey.PublicKey, state)
	assert.NoError(t, err)

	// Try to add the same transaction again (should fail)
	err = txPool.AddTransaction(tx, &privKey.PublicKey, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "transaction already in pool")

	// Apply transaction to state
	err = state.ApplyTransaction(tx)
	assert.NoError(t, err)

	// Try to add transaction with same nonce again (should fail)
	err = txPool.AddTransaction(tx, &privKey.PublicKey, state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid nonce")
}
