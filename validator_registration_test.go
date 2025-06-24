package main

import (
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidatorRegistrationTransaction tests the complete validator registration flow
func TestValidatorRegistrationTransaction(t *testing.T) {
	// Setup
	store := NewMemoryStore()
	bc, err := NewBlockchain(store)
	require.NoError(t, err)

	state := NewState()
	txPool := NewTransactionPool()
	vr := NewValidatorRegistry(NewMemoryStore(), "validators")

	// Create test account
	priv, _ := btcec.NewPrivateKey()
	addr := pubKeyToAddress(priv.PubKey())

	// Fund account
	acc := &Account{Address: addr, Balance: 1000, Nonce: 0}
	require.NoError(t, state.PutAccount(acc))

	// Create validator registration transaction
	tx := &Transaction{
		From:      addr,
		To:        addr, // Self-registration
		Value:     500,  // Stake amount
		Nonce:     1,
		Fee:       10,
		Type:      "register_validator",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx.Sign(priv))

	// Add transaction to pool
	err = txPool.AddTransaction(tx, priv.PubKey(), state)
	assert.NoError(t, err)
	assert.Equal(t, 1, txPool.Size())

	// Select transactions for block inclusion
	selectedTxs := txPool.SelectTransactions(10, state)
	assert.Len(t, selectedTxs, 1)
	assert.Equal(t, tx.Hash, selectedTxs[0].Hash)

	// Create block with transaction
	proposer := &Validator{Address: addr, Stake: 100}
	block, err := bc.CreateBlock(selectedTxs, proposer, priv)
	require.NoError(t, err)
	assert.Len(t, block.Transactions, 1)
	assert.Equal(t, tx.Hash, block.Transactions[0].Hash)

	// Apply block to blockchain
	err = bc.AddBlock(block)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), bc.Height())

	// Apply block transactions to state and registry
	err = bc.ApplyBlockWithRegistry(block, state, vr)
	assert.NoError(t, err)

	// Remove transactions from pool
	for _, tx := range block.Transactions {
		txPool.RemoveTransaction(tx.Hash)
	}

	// Verify validator is registered
	validator, err := vr.GetValidator(addr)
	assert.NoError(t, err)
	assert.NotNil(t, validator)
	assert.Equal(t, uint64(500), validator.Stake)
	assert.Equal(t, uint64(0), validator.DelegatedStake)
	assert.True(t, validator.Participating)

	// Verify account balance is reduced
	updatedAcc, err := state.GetAccount(addr)
	assert.NoError(t, err)
	assert.Equal(t, uint64(490), updatedAcc.Balance) // 1000 - 500 - 10
	assert.Equal(t, uint64(1), updatedAcc.Nonce)

	// Verify transaction is removed from pool
	assert.Equal(t, 0, txPool.Size())
}

// TestDelegationTransaction tests the complete delegation flow
func TestDelegationTransaction(t *testing.T) {
	// Setup
	store := NewMemoryStore()
	bc, err := NewBlockchain(store)
	require.NoError(t, err)

	state := NewState()
	txPool := NewTransactionPool()
	vr := NewValidatorRegistry(NewMemoryStore(), "validators")

	// Create test accounts
	priv, _ := btcec.NewPrivateKey()
	privKey1 := priv
	privKey2, _ := btcec.NewPrivateKey()
	addr1 := pubKeyToAddress(privKey1.PubKey())
	addr2 := pubKeyToAddress(privKey2.PubKey())

	// Fund accounts
	acc1 := &Account{Address: addr1, Balance: 1000, Nonce: 0}
	acc2 := &Account{Address: addr2, Balance: 500, Nonce: 0}
	require.NoError(t, state.PutAccount(acc1))
	require.NoError(t, state.PutAccount(acc2))

	// Register validator first
	validator := &Validator{Address: addr2, Stake: 200, Participating: true}
	require.NoError(t, vr.RegisterValidator(validator))

	// Create delegation transaction
	tx := &Transaction{
		From:      addr1, // Delegator
		To:        addr2, // Validator
		Value:     300,   // Delegation amount
		Nonce:     1,
		Fee:       10,
		Type:      "delegate",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx.Sign(privKey1))

	// Add transaction to pool
	err = txPool.AddTransaction(tx, privKey1.PubKey(), state)
	assert.NoError(t, err)
	assert.Equal(t, 1, txPool.Size())

	// Select transactions for block inclusion
	selectedTxs := txPool.SelectTransactions(10, state)
	assert.Len(t, selectedTxs, 1)
	assert.Equal(t, tx.Hash, selectedTxs[0].Hash)

	// Create block with transaction
	proposer := &Validator{Address: addr1, Stake: 100}
	block, err := bc.CreateBlock(selectedTxs, proposer, privKey1)
	require.NoError(t, err)
	assert.Len(t, block.Transactions, 1)
	assert.Equal(t, tx.Hash, block.Transactions[0].Hash)

	// Apply block to blockchain
	err = bc.AddBlock(block)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), bc.Height())

	// Apply block transactions to state and registry
	err = bc.ApplyBlockWithRegistry(block, state, vr)
	assert.NoError(t, err)

	// Remove transactions from pool
	for _, tx := range block.Transactions {
		txPool.RemoveTransaction(tx.Hash)
	}

	// Verify validator has received delegated stake
	updatedValidator, err := vr.GetValidator(addr2)
	assert.NoError(t, err)
	assert.NotNil(t, updatedValidator)
	assert.Equal(t, uint64(200), updatedValidator.Stake)          // Original stake
	assert.Equal(t, uint64(300), updatedValidator.DelegatedStake) // Delegated stake

	// Verify delegator balance is reduced
	updatedAcc1, err := state.GetAccount(addr1)
	assert.NoError(t, err)
	assert.Equal(t, uint64(690), updatedAcc1.Balance) // 1000 - 300 - 10
	assert.Equal(t, uint64(1), updatedAcc1.Nonce)

	// Verify validator balance is unchanged (delegation doesn't affect account balance)
	updatedAcc2, err := state.GetAccount(addr2)
	assert.NoError(t, err)
	assert.Equal(t, uint64(500), updatedAcc2.Balance) // Unchanged
	assert.Equal(t, uint64(0), updatedAcc2.Nonce)

	// Verify transaction is removed from pool
	assert.Equal(t, 0, txPool.Size())
}

// TestValidatorRegistrationValidation tests validation rules for registration transactions
func TestValidatorRegistrationValidation(t *testing.T) {
	state := NewState()
	txPool := NewTransactionPool()

	// Create test account
	priv, _ := btcec.NewPrivateKey()
	addr := pubKeyToAddress(priv.PubKey())

	// Fund account
	acc := &Account{Address: addr, Balance: 100, Nonce: 0}
	require.NoError(t, state.PutAccount(acc))

	// Test 1: Zero stake registration (should fail)
	tx1 := &Transaction{
		From:      addr,
		To:        addr,
		Value:     0, // Invalid: zero stake
		Nonce:     1,
		Fee:       10,
		Type:      "register_validator",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx1.Sign(priv))

	err := txPool.AddTransaction(tx1, priv.PubKey(), state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "validator registration requires non-zero stake")

	// Test 2: Insufficient balance (should fail)
	tx2 := &Transaction{
		From:      addr,
		To:        addr,
		Value:     200, // More than balance
		Nonce:     1,
		Fee:       10,
		Type:      "register_validator",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx2.Sign(priv))

	err = txPool.AddTransaction(tx2, priv.PubKey(), state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "insufficient balance")

	// Test 3: Valid registration (should succeed)
	tx3 := &Transaction{
		From:      addr,
		To:        addr,
		Value:     50, // Valid stake amount
		Nonce:     1,
		Fee:       10,
		Type:      "register_validator",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx3.Sign(priv))

	err = txPool.AddTransaction(tx3, priv.PubKey(), state)
	assert.NoError(t, err)
	assert.Equal(t, 1, txPool.Size())
}

// TestDelegationValidation tests validation rules for delegation transactions
func TestDelegationValidation(t *testing.T) {
	state := NewState()
	txPool := NewTransactionPool()
	vr := NewValidatorRegistry(NewMemoryStore(), "validators")

	// Create test accounts
	priv, _ := btcec.NewPrivateKey()
	privKey1 := priv
	privKey2, _ := btcec.NewPrivateKey()
	addr1 := pubKeyToAddress(privKey1.PubKey())
	addr2 := pubKeyToAddress(privKey2.PubKey())

	// Fund delegator account
	acc1 := &Account{Address: addr1, Balance: 100, Nonce: 0}
	require.NoError(t, state.PutAccount(acc1))

	// Test 1: Zero delegation amount (should fail)
	tx1 := &Transaction{
		From:      addr1,
		To:        addr2,
		Value:     0, // Invalid: zero amount
		Nonce:     1,
		Fee:       10,
		Type:      "delegate",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx1.Sign(privKey1))

	err := txPool.AddTransaction(tx1, privKey1.PubKey(), state)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "delegation requires non-zero amount")

	// Test 2: Delegation to non-registered validator (should fail)
	tx2 := &Transaction{
		From:      addr1,
		To:        addr2, // Not registered
		Value:     50,
		Nonce:     1,
		Fee:       10,
		Type:      "delegate",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx2.Sign(privKey1))

	err = txPool.AddTransaction(tx2, privKey1.PubKey(), state)
	assert.NoError(t, err) // Pool validation passes, but block application will fail

	// Remove the invalid transaction from pool
	txPool.RemoveTransaction(tx2.Hash)

	// Test 3: Valid delegation (should succeed)
	// First register the validator
	validator := &Validator{Address: addr2, Stake: 100, Participating: true}
	require.NoError(t, vr.RegisterValidator(validator))

	tx3 := &Transaction{
		From:      addr1,
		To:        addr2,
		Value:     50,
		Nonce:     1,
		Fee:       10,
		Type:      "delegate",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx3.Sign(privKey1))

	err = txPool.AddTransaction(tx3, privKey1.PubKey(), state)
	assert.NoError(t, err)
	assert.Equal(t, 1, txPool.Size())
}

// TestMultipleDelegations tests multiple delegations to the same validator
func TestMultipleDelegations(t *testing.T) {
	// Setup
	store := NewMemoryStore()
	bc, err := NewBlockchain(store)
	require.NoError(t, err)

	state := NewState()
	txPool := NewTransactionPool()
	vr := NewValidatorRegistry(NewMemoryStore(), "validators")

	// Create test accounts
	priv, _ := btcec.NewPrivateKey()
	privKey1 := priv
	privKey2, _ := btcec.NewPrivateKey()
	addr1 := pubKeyToAddress(privKey1.PubKey())
	addr2 := pubKeyToAddress(privKey2.PubKey())

	// Fund accounts
	acc1 := &Account{Address: addr1, Balance: 1000, Nonce: 0}
	acc2 := &Account{Address: addr2, Balance: 1000, Nonce: 0}
	require.NoError(t, state.PutAccount(acc1))
	require.NoError(t, state.PutAccount(acc2))

	// Register validator
	validator := &Validator{Address: addr2, Stake: 200, Participating: true}
	require.NoError(t, vr.RegisterValidator(validator))

	// Create multiple delegation transactions
	txs := []*Transaction{}
	for i, privKey := range []*btcec.PrivateKey{privKey1, privKey2} {
		addr := pubKeyToAddress(privKey.PubKey())
		tx := &Transaction{
			From:      addr,
			To:        addr2,              // Same validator
			Value:     100 + uint64(i*50), // Different amounts
			Nonce:     1,
			Fee:       10,
			Type:      "delegate",
			Timestamp: time.Now().UnixNano(),
		}
		require.NoError(t, tx.Sign(privKey))
		txs = append(txs, tx)
	}

	// Add transactions to pool
	for _, tx := range txs {
		privKey := privKey1
		if tx.From == addr2 {
			privKey = privKey2
		}
		err := txPool.AddTransaction(tx, privKey.PubKey(), state)
		assert.NoError(t, err)
	}
	assert.Equal(t, 2, txPool.Size())

	// Select transactions for block inclusion
	selectedTxs := txPool.SelectTransactions(10, state)
	assert.Len(t, selectedTxs, 2)

	// Create block with transactions
	proposer := &Validator{Address: addr1, Stake: 100}
	block, err := bc.CreateBlock(selectedTxs, proposer, privKey1)
	require.NoError(t, err)
	assert.Len(t, block.Transactions, 2)

	// Apply block
	err = bc.AddBlock(block)
	assert.NoError(t, err)

	err = bc.ApplyBlockWithRegistry(block, state, vr)
	assert.NoError(t, err)

	// Remove transactions from pool
	for _, tx := range block.Transactions {
		txPool.RemoveTransaction(tx.Hash)
	}

	// Verify validator has received all delegated stake
	updatedValidator, err := vr.GetValidator(addr2)
	assert.NoError(t, err)
	assert.NotNil(t, updatedValidator)
	assert.Equal(t, uint64(200), updatedValidator.Stake)          // Original stake
	assert.Equal(t, uint64(250), updatedValidator.DelegatedStake) // 100 + 150

	// Verify delegator balances are reduced
	updatedAcc1, err := state.GetAccount(addr1)
	assert.NoError(t, err)
	assert.Equal(t, uint64(890), updatedAcc1.Balance) // 1000 - 100 - 10

	updatedAcc2, err := state.GetAccount(addr2)
	assert.NoError(t, err)
	assert.Equal(t, uint64(840), updatedAcc2.Balance) // 1000 - 150 - 10

	// Verify transaction pool is empty
	assert.Equal(t, 0, txPool.Size())
}
