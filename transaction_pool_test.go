package main

import (
	"testing"
	"time"

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

func TestTransactionBatching(t *testing.T) {
	pool := NewTransactionPool()
	state := NewState()

	// Create test transactions with different priorities
	privKeys := make([]*btcec.PrivateKey, 3)
	fromAddrs := make([]Address, 3)
	for i := 0; i < 3; i++ {
		privKeys[i], _ = btcec.NewPrivateKey()
		fromAddrs[i] = pubKeyToAddress(privKeys[i].PubKey())
	}
	txs := []*Transaction{
		{
			From:      fromAddrs[0],
			To:        Address{2},
			Value:     100,
			Fee:       10, // High fee ratio
			Nonce:     1,
			Type:      "transfer",
			Timestamp: time.Now().UnixNano(),
		},
		{
			From:      fromAddrs[1],
			To:        Address{4},
			Value:     1000,
			Fee:       5, // Low fee ratio
			Nonce:     1,
			Type:      "transfer",
			Timestamp: time.Now().UnixNano(),
		},
		{
			From:      fromAddrs[2],
			To:        Address{6},
			Value:     100, // Non-zero stake for validator registration
			Fee:       50,
			Nonce:     1,
			Type:      "register_validator", // High priority type
			Timestamp: time.Now().UnixNano(),
		},
	}

	// Fund the correct From addresses
	for i := 0; i < 3; i++ {
		balance := uint64(20000)
		account := &Account{
			Address: fromAddrs[i],
			Balance: balance,
			Nonce:   0,
		}
		state.PutAccount(account)
	}

	// Add transactions to pool
	for i, tx := range txs {
		// Use the correct signature method from existing tests
		data, _ := tx.Encode()
		tx.Hash = sha3.Sum256(data)
		sig := ecdsa.Sign(privKeys[i], tx.Hash[:])
		tx.Signature = sig.Serialize()

		pubKey := privKeys[i].PubKey()
		err := pool.AddTransaction(tx, pubKey, state)
		assert.NoError(t, err)
	}

	// Test optimized batch creation
	batch := pool.CreateOptimizedBatch(state)
	assert.NotNil(t, batch)
	assert.Greater(t, len(batch.Transactions), 0)
	assert.Greater(t, batch.TotalFee, uint64(0))
	assert.Greater(t, batch.Priority, 0.0)

	// Test that validator registration has higher priority
	validatorTxFound := false
	for _, tx := range batch.Transactions {
		if tx.Type == "register_validator" {
			validatorTxFound = true
			break
		}
	}
	assert.True(t, validatorTxFound, "Validator registration should be included in batch")

	// Test batch statistics
	stats := pool.GetBatchStatistics()
	assert.NotNil(t, stats)
	assert.Equal(t, 3, stats["total_transactions"])
	assert.Equal(t, 3, stats["valid_transactions"])
	assert.Equal(t, 0, stats["used_transactions"])
	assert.Greater(t, stats["total_fees"], uint64(0))
}

func TestTransactionPriorityCalculation(t *testing.T) {
	pool := NewTransactionPool()

	// Test high fee ratio transaction
	highFeeTx := &Transaction{
		From:      Address{1},
		To:        Address{2},
		Value:     100,
		Fee:       20, // 20% fee ratio
		Type:      "transfer",
		Timestamp: time.Now().UnixNano(),
	}

	// Test low fee ratio transaction
	lowFeeTx := &Transaction{
		From:      Address{3},
		To:        Address{4},
		Value:     1000,
		Fee:       5, // 0.5% fee ratio
		Type:      "transfer",
		Timestamp: time.Now().UnixNano(),
	}

	// Test validator registration
	validatorTx := &Transaction{
		From:      Address{5},
		To:        Address{6},
		Value:     0,
		Fee:       50,
		Type:      "register_validator",
		Timestamp: time.Now().UnixNano(),
	}

	highPriority := pool.calculatePriority(highFeeTx)
	lowPriority := pool.calculatePriority(lowFeeTx)
	validatorPriority := pool.calculatePriority(validatorTx)

	// High fee ratio should have higher priority than low fee ratio
	assert.Greater(t, highPriority, lowPriority)

	// Validator registration should have high priority due to type multiplier
	assert.Greater(t, validatorPriority, lowPriority)
}

func TestBatchConstraints(t *testing.T) {
	pool := NewTransactionPool()
	state := NewState()

	// Set small batch constraints
	pool.SetBatchingConfig(2, 100, 5000000000) // Max 2 txs, max 100 fee

	// Create accounts
	for i := 0; i < 10; i++ {
		addr := Address{byte(i + 1)}
		account := &Account{
			Address: addr,
			Balance: 10000,
			Nonce:   0,
		}
		state.PutAccount(account)
	}

	// Add many transactions
	for i := 0; i < 10; i++ {
		tx := &Transaction{
			From:      Address{1},
			To:        Address{byte(i + 2)},
			Value:     100,
			Fee:       50,
			Nonce:     1,
			Type:      "transfer",
			Timestamp: time.Now().UnixNano(),
		}

		privKey, _ := btcec.NewPrivateKey()

		// Use the correct signature method from existing tests
		data, _ := tx.Encode()
		tx.Hash = sha3.Sum256(data)
		sig := ecdsa.Sign(privKey, tx.Hash[:])
		tx.Signature = sig.Serialize()

		pubKey := privKey.PubKey()
		pool.AddTransaction(tx, pubKey, state)
	}

	// Test that batch respects constraints
	batch := pool.CreateOptimizedBatch(state)
	assert.LessOrEqual(t, len(batch.Transactions), 2)
	assert.LessOrEqual(t, batch.TotalFee, uint64(100))
}

func TestTransactionSelectionWithPriority(t *testing.T) {
	pool := NewTransactionPool()
	state := NewState()

	// Create private keys and corresponding addresses
	privKey1, _ := btcec.NewPrivateKey()
	privKey2, _ := btcec.NewPrivateKey()
	privKey3, _ := btcec.NewPrivateKey()

	addr1 := pubKeyToAddress(privKey1.PubKey())
	addr2 := pubKeyToAddress(privKey2.PubKey())
	addr3 := pubKeyToAddress(privKey3.PubKey())

	// Create and fund accounts
	accounts := []*Account{
		{Address: addr1, Balance: 10000, Nonce: 0},
		{Address: addr2, Balance: 10000, Nonce: 0},
		{Address: addr3, Balance: 10000, Nonce: 0},
	}

	for _, account := range accounts {
		state.PutAccount(account)
	}

	// Add transactions with different priorities
	txs := []*Transaction{
		{From: addr1, To: Address{2}, Value: 100, Fee: 5, Nonce: 1, Type: "transfer", Timestamp: time.Now().UnixNano()},
		{From: addr2, To: Address{4}, Value: 100, Fee: 20, Nonce: 1, Type: "transfer", Timestamp: time.Now().UnixNano()},           // Higher fee
		{From: addr3, To: Address{6}, Value: 100, Fee: 30, Nonce: 1, Type: "register_validator", Timestamp: time.Now().UnixNano()}, // Highest priority
	}

	privKeys := []*btcec.PrivateKey{privKey1, privKey2, privKey3}

	for i, tx := range txs {
		// Use the correct signature method from existing tests
		data, _ := tx.Encode()
		tx.Hash = sha3.Sum256(data)
		sig := ecdsa.Sign(privKeys[i], tx.Hash[:])
		tx.Signature = sig.Serialize()

		pubKey := privKeys[i].PubKey()
		err := pool.AddTransaction(tx, pubKey, state)
		assert.NoError(t, err)
	}

	// Test priority-based selection
	selected := pool.SelectTransactions(2, state)
	assert.Equal(t, 2, len(selected))

	// First transaction should be validator registration (highest priority)
	assert.Equal(t, "register_validator", selected[0].Type)

	// Second transaction should be high fee transaction
	assert.Equal(t, uint64(20), selected[1].Fee)
}
