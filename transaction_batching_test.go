package main

import (
	"crypto/sha3"
	"encoding/json"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
)

func TestTransactionBatcher_BatchBySize(t *testing.T) {
	pool := NewTransactionPool()
	// Set batch config to allow all test transactions
	pool.SetBatchingConfig(10, 1000000, 5000000000)
	batcher := NewTransactionBatcher(pool, 3, 10*time.Second)
	state := NewState()

	// Create test transactions with different private keys
	privKeys := make([]*btcec.PrivateKey, 5)
	fromAddrs := make([]Address, 5)

	for i := 0; i < 5; i++ {
		privKeys[i], _ = btcec.NewPrivateKey()
		fromAddrs[i] = pubKeyToAddress(privKeys[i].PubKey())

		// Initialize account with sufficient balance and correct nonce
		acc := &Account{Address: fromAddrs[i], Balance: 1000000, Nonce: 0}
		state.PutAccount(acc)

		tx := &Transaction{
			From:      fromAddrs[i], // Use the address derived from the public key
			To:        Address{0xFF},
			Value:     uint64(100 + i*10), // Small values to ensure sufficient balance
			Nonce:     1,                  // Match account nonce + 1
			Fee:       uint64(10 + i*5),   // Small fees
			Timestamp: time.Now().UnixNano(),
			Type:      "transfer",
		}
		// Calculate hash
		data, _ := json.Marshal(tx)
		tx.Hash = sha3.Sum256(data)
		// Sign transaction with the corresponding private key
		tx.Sign(privKeys[i])
		err := pool.AddTransaction(tx, privKeys[i].PubKey(), state)
		if err != nil {
			t.Logf("Failed to add transaction %d: %v", i, err)
		} else {
			t.Logf("Successfully added transaction %d", i)
		}
	}

	// Get next batch
	batch := batcher.NextBatch(state)
	t.Logf("Pool size: %d", pool.Size())
	t.Logf("Pool stats: %+v", pool.GetBatchStatistics())
	t.Logf("Batch size: %d", len(batch))
	if len(batch) != 3 {
		t.Errorf("Expected batch size 3, got %d", len(batch))
	}

	// Get remaining batch
	batch2 := batcher.NextBatch(state)
	t.Logf("Second batch size: %d", len(batch2))
	if len(batch2) != 2 {
		t.Errorf("Expected batch size 2, got %d", len(batch2))
	}
}

func TestTransactionBatcher_BatchByTimeout(t *testing.T) {
	pool := NewTransactionPool()
	// Set batch config to allow all test transactions
	pool.SetBatchingConfig(10, 1000000, 5000000000)
	batcher := NewTransactionBatcher(pool, 10, 100*time.Millisecond)
	state := NewState()

	// Create a test private key
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatalf("Failed to generate test private key: %v", err)
	}

	// Get the address from the public key
	fromAddr := pubKeyToAddress(privKey.PubKey())

	// Add one transaction
	tx := &Transaction{
		From:      fromAddr, // Use the address derived from the public key
		To:        Address{0x02},
		Value:     100,
		Nonce:     1,
		Fee:       10,
		Timestamp: time.Now().UnixNano(),
		Type:      "transfer",
	}
	data, _ := json.Marshal(tx)
	tx.Hash = sha3.Sum256(data)
	tx.Sign(privKey)
	// Initialize account with sufficient balance and correct nonce
	acc := &Account{Address: fromAddr, Balance: 1000000, Nonce: 0}
	state.PutAccount(acc)
	err = pool.AddTransaction(tx, privKey.PubKey(), state)
	if err != nil {
		t.Logf("Failed to add transaction: %v", err)
	} else {
		t.Logf("Successfully added transaction")
	}

	// Get batch (should return the one transaction)
	batch := batcher.NextBatch(state)
	t.Logf("Pool size: %d", pool.Size())
	t.Logf("Pool stats: %+v", pool.GetBatchStatistics())
	t.Logf("Batch size: %d", len(batch))
	if len(batch) != 1 {
		t.Errorf("Expected batch size 1, got %d", len(batch))
	}
}
