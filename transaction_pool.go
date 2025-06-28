package main

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
)

// TransactionBatch represents a batch of transactions optimized for block inclusion
type TransactionBatch struct {
	Transactions []*Transaction
	TotalFee     uint64
	TotalSize    int
	Priority     float64 // Higher priority batches are selected first
}

// TransactionPool with enhanced batching capabilities
type TransactionPool struct {
	mu           sync.RWMutex
	transactions map[Hash]*Transaction

	// Batching configuration
	maxBatchSize int
	maxBatchFee  uint64
	batchTimeout int64 // nanoseconds

	// Priority tracking
	priorityScores map[Hash]float64
}

func NewTransactionPool() *TransactionPool {
	return &TransactionPool{
		transactions:   make(map[Hash]*Transaction),
		maxBatchSize:   100,        // Maximum transactions per batch
		maxBatchFee:    1000,       // Maximum total fee per batch
		batchTimeout:   5000000000, // 5 seconds in nanoseconds
		priorityScores: make(map[Hash]float64),
	}
}

// SetBatchingConfig allows configuration of batching parameters
func (tp *TransactionPool) SetBatchingConfig(maxSize int, maxFee uint64, timeout int64) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	tp.maxBatchSize = maxSize
	tp.maxBatchFee = maxFee
	tp.batchTimeout = timeout
}

// calculatePriority calculates a priority score for a transaction
func (tp *TransactionPool) calculatePriority(tx *Transaction) float64 {
	// Base priority on fee-to-value ratio
	if tx.Value == 0 {
		return float64(tx.Fee) // For non-transfer transactions
	}

	// Higher fee-to-value ratio = higher priority
	feeRatio := float64(tx.Fee) / float64(tx.Value)

	// Age factor: older transactions get slight priority boost
	age := float64(time.Now().UnixNano()-tx.Timestamp) / float64(tp.batchTimeout)
	if age > 1.0 {
		age = 1.0 // Cap at 100% boost
	}

	// Type priority: validator registrations and delegations get higher priority
	typeMultiplier := 1.0
	switch tx.Type {
	case "register_validator":
		typeMultiplier = 2.0
	case "delegate":
		typeMultiplier = 1.5
	case "participation":
		typeMultiplier = 1.2
	}

	return (feeRatio + age*0.1) * typeMultiplier
}

// AddTransaction validates and adds a transaction to the pool.
func (tp *TransactionPool) AddTransaction(tx *Transaction, pubKey *btcec.PublicKey, state *State) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// 1. Verify Signature
	if !tx.Verify(pubKey) {
		log.Printf("DEBUG: Signature verification failed for tx %s", tx.Hash.ToHex())
		return errors.New("invalid signature")
	}

	// Handle different transaction types
	switch tx.Type {
	case "participation":
		// Only check for duplicates
		if _, ok := tp.transactions[tx.Hash]; ok {
			return errors.New("transaction already in pool")
		}
		tp.transactions[tx.Hash] = tx
		log.Printf("Added participation transaction to pool: %x", tx.Hash)
		return nil
	case "register_validator":
		// Check for duplicates
		if _, ok := tp.transactions[tx.Hash]; ok {
			return errors.New("transaction already in pool")
		}
		// Validate stake amount
		if tx.Value == 0 {
			return errors.New("validator registration requires non-zero stake")
		}
		// Check nonce and balance from state
		senderAddr := pubKeyToAddress(pubKey)
		sender, err := state.GetAccount(senderAddr)
		if err != nil {
			// For new accounts, create them with 0 balance and nonce 0
			sender = &Account{Address: senderAddr, Balance: 0, Nonce: 0}
			if err := state.PutAccount(sender); err != nil {
				return fmt.Errorf("failed to create new account: %w", err)
			}
		}

		if tx.Nonce != sender.Nonce+1 {
			return fmt.Errorf("invalid nonce. got %d, want %d", tx.Nonce, sender.Nonce+1)
		}

		if tx.Value+tx.Fee > sender.Balance {
			return fmt.Errorf("insufficient balance. want %d, have %d", tx.Value+tx.Fee, sender.Balance)
		}

		tp.transactions[tx.Hash] = tx
		log.Printf("Added validator registration transaction to pool: %x", tx.Hash)
		return nil
	case "delegate":
		// Check for duplicates
		if _, ok := tp.transactions[tx.Hash]; ok {
			return errors.New("transaction already in pool")
		}
		// Validate delegation amount
		if tx.Value == 0 {
			return errors.New("delegation requires non-zero amount")
		}
		// Check nonce and balance from state
		senderAddr := pubKeyToAddress(pubKey)
		sender, err := state.GetAccount(senderAddr)
		if err != nil {
			// For new accounts, create them with 0 balance and nonce 0
			sender = &Account{Address: senderAddr, Balance: 0, Nonce: 0}
			if err := state.PutAccount(sender); err != nil {
				return fmt.Errorf("failed to create new account: %w", err)
			}
		}

		if tx.Nonce != sender.Nonce+1 {
			return fmt.Errorf("invalid nonce. got %d, want %d", tx.Nonce, sender.Nonce+1)
		}

		if tx.Value+tx.Fee > sender.Balance {
			return fmt.Errorf("insufficient balance. want %d, have %d", tx.Value+tx.Fee, sender.Balance)
		}

		tp.transactions[tx.Hash] = tx
		log.Printf("Added delegation transaction to pool: %x", tx.Hash)
		return nil
	default:
		// Standard transfer transaction validation
		// 2. Check nonce and balance from state
		senderAddr := pubKeyToAddress(pubKey)
		sender, err := state.GetAccount(senderAddr)
		if err != nil {
			// For new accounts, create them with 0 balance and nonce 0
			sender = &Account{Address: senderAddr, Balance: 0, Nonce: 0}
			if err := state.PutAccount(sender); err != nil {
				return fmt.Errorf("failed to create new account: %w", err)
			}
		}

		if tx.Nonce != sender.Nonce+1 {
			return fmt.Errorf("invalid nonce. got %d, want %d", tx.Nonce, sender.Nonce+1)
		}

		if tx.Value+tx.Fee > sender.Balance {
			return fmt.Errorf("insufficient balance. want %d, have %d", tx.Value+tx.Fee, sender.Balance)
		}

		// 3. Check for duplicates
		if _, ok := tp.transactions[tx.Hash]; ok {
			return errors.New("transaction already in pool")
		}

		tp.transactions[tx.Hash] = tx
		log.Printf("Added transaction to pool: %x", tx.Hash)
		return nil
	}
}

// RemoveTransaction removes a transaction from the pool by hash.
func (tp *TransactionPool) RemoveTransaction(hash [32]byte) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	delete(tp.transactions, hash)
}

// SelectTransactions selects up to n transactions for block inclusion, filtering by nonce and balance at selection time.
func (tp *TransactionPool) SelectTransactions(n int, state *State) []*Transaction {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	log.Printf("DEBUG: SelectTransactions called with n=%d, pool size=%d", n, len(tp.transactions))

	// Create a slice of valid transactions with their priorities
	type txWithPriority struct {
		tx       *Transaction
		priority float64
	}

	var validTxs []txWithPriority

	for _, tx := range tp.transactions {
		// Skip used transactions
		if tx.Used {
			log.Printf("DEBUG: Skipping tx %s: already used", tx.Hash.ToHex())
			continue
		}

		// Check nonce and balance at selection time
		sender, err := tp.getSenderAccount(tx, state)
		if err != nil {
			log.Printf("DEBUG: Skipping tx %s: could not get sender account: %v", tx.Hash.ToHex(), err)
			continue
		}
		if tx.Nonce != sender.Nonce+1 {
			log.Printf("DEBUG: Skipping tx %s: invalid nonce at selection. got %d, want %d", tx.Hash.ToHex(), tx.Nonce, sender.Nonce+1)
			continue
		}
		if tx.Value+tx.Fee > sender.Balance {
			log.Printf("DEBUG: Skipping tx %s: insufficient balance at selection. want %d, have %d", tx.Hash.ToHex(), tx.Value+tx.Fee, sender.Balance)
			continue
		}

		// Calculate priority for this transaction
		priority := tp.calculatePriority(tx)
		validTxs = append(validTxs, txWithPriority{tx: tx, priority: priority})
	}

	// Sort by priority (highest first)
	sort.Slice(validTxs, func(i, j int) bool {
		return validTxs[i].priority > validTxs[j].priority
	})

	// Select top n transactions
	txs := make([]*Transaction, 0, n)
	for i, txWithPrio := range validTxs {
		if i >= n {
			break
		}
		log.Printf("DEBUG: Selecting transaction %s (To: %s, Value: %d, Fee: %d, Priority: %.3f)",
			txWithPrio.tx.Hash.ToHex(), txWithPrio.tx.To.ToHex(), txWithPrio.tx.Value, txWithPrio.tx.Fee, txWithPrio.priority)
		txs = append(txs, txWithPrio.tx)
	}

	log.Printf("DEBUG: SelectTransactions returning %d transactions", len(txs))
	return txs
}

// CreateOptimizedBatch creates an optimized batch of transactions for block inclusion
func (tp *TransactionPool) CreateOptimizedBatch(state *State) *TransactionBatch {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	// Get all valid transactions with priorities
	type txWithPriority struct {
		tx       *Transaction
		priority float64
	}

	var validTxs []txWithPriority

	for _, tx := range tp.transactions {
		if tx.Used {
			continue
		}

		// Validate transaction
		sender, err := tp.getSenderAccount(tx, state)
		if err != nil {
			continue
		}
		if tx.Nonce != sender.Nonce+1 {
			continue
		}
		if tx.Value+tx.Fee > sender.Balance {
			continue
		}

		priority := tp.calculatePriority(tx)
		validTxs = append(validTxs, txWithPriority{tx: tx, priority: priority})
	}

	// Sort by priority
	sort.Slice(validTxs, func(i, j int) bool {
		return validTxs[i].priority > validTxs[j].priority
	})

	// Create optimized batch using greedy algorithm
	var batch []*Transaction
	totalFee := uint64(0)
	totalSize := 0

	for _, txWithPrio := range validTxs {
		tx := txWithPrio.tx

		// Check batch constraints
		if len(batch) >= tp.maxBatchSize {
			break
		}
		if totalFee+tx.Fee > tp.maxBatchFee {
			continue
		}

		// Estimate transaction size (simplified)
		txSize := 200                   // Base size estimate
		if totalSize+txSize > 1000000 { // 1MB limit
			continue
		}

		batch = append(batch, tx)
		totalFee += tx.Fee
		totalSize += txSize
	}

	// Calculate batch priority (average of transaction priorities)
	batchPriority := 0.0
	if len(batch) > 0 {
		for _, tx := range batch {
			batchPriority += tp.calculatePriority(tx)
		}
		batchPriority /= float64(len(batch))
	}

	return &TransactionBatch{
		Transactions: batch,
		TotalFee:     totalFee,
		TotalSize:    totalSize,
		Priority:     batchPriority,
	}
}

// GetBatchStatistics returns statistics about the current transaction pool
func (tp *TransactionPool) GetBatchStatistics() map[string]interface{} {
	tp.mu.RLock()
	defer tp.mu.RUnlock()

	totalFee := uint64(0)
	validCount := 0
	usedCount := 0

	for _, tx := range tp.transactions {
		if tx.Used {
			usedCount++
		} else {
			validCount++
			totalFee += tx.Fee
		}
	}

	return map[string]interface{}{
		"total_transactions": len(tp.transactions),
		"valid_transactions": validCount,
		"used_transactions":  usedCount,
		"total_fees":         totalFee,
		"average_fee":        float64(totalFee) / float64(validCount),
		"max_batch_size":     tp.maxBatchSize,
		"max_batch_fee":      tp.maxBatchFee,
	}
}

// getSenderAccount is a helper to get the sender's account for a transaction using the provided state
func (tp *TransactionPool) getSenderAccount(tx *Transaction, state *State) (*Account, error) {
	return state.GetAccount(tx.From)
}

// Size returns the number of transactions in the pool.
func (tp *TransactionPool) Size() int {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	return len(tp.transactions)
}

// GetTransactions returns all transactions from the pool.
func (tp *TransactionPool) GetTransactions() []*Transaction {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	txs := make([]*Transaction, 0, len(tp.transactions))
	for _, tx := range tp.transactions {
		txs = append(txs, tx)
	}
	return txs
}

// MarkTransactionAsUsed marks a transaction as used to prevent duplicate inclusion
func (tp *TransactionPool) MarkTransactionAsUsed(hash Hash) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	if tx, exists := tp.transactions[hash]; exists {
		tx.Used = true
		log.Printf("DEBUG: Marked transaction %s as used", hash.ToHex())
	}
}
