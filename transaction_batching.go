package main

import (
	"sync"
	"time"
)

// TransactionBatcher manages transaction batching for efficient block production
type TransactionBatcher struct {
	pool      *TransactionPool
	batchSize int
	timeout   time.Duration
	mu        sync.Mutex
	batches   []*TransactionBatch
	metrics   *BatchMetrics
}

// BatchMetrics tracks batch performance metrics
type BatchMetrics struct {
	TotalBatches   int
	AverageSize    float64
	AverageFee     float64
	ProcessingTime time.Duration
	SuccessRate    float64
	TotalProcessed int
}

func NewTransactionBatcher(pool *TransactionPool, batchSize int, timeout time.Duration) *TransactionBatcher {
	return &TransactionBatcher{
		pool:      pool,
		batchSize: batchSize,
		timeout:   timeout,
		batches:   make([]*TransactionBatch, 0),
		metrics:   &BatchMetrics{},
	}
}

// AddTransaction adds a transaction to the current batch
func (tb *TransactionBatcher) AddTransaction(tx *Transaction) {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	// Implementation would add transaction to current batch
}

// NextBatch returns the next batch of transactions using the pool's optimized batch logic
func (tb *TransactionBatcher) NextBatch(state *State) []*Transaction {
	tb.mu.Lock()
	defer tb.mu.Unlock()

	batch := tb.pool.CreateOptimizedBatch(state)
	if batch == nil || len(batch.Transactions) == 0 {
		return nil
	}

	// Enforce the batcher's own batchSize limit
	max := tb.batchSize
	if len(batch.Transactions) < max {
		max = len(batch.Transactions)
	}
	selected := batch.Transactions[:max]

	// Mark transactions as used
	for _, tx := range selected {
		tb.pool.MarkTransactionAsUsed(tx.Hash)
	}

	return selected
}

// Metrics returns batch performance metrics
func (tb *TransactionBatcher) Metrics() *BatchMetrics {
	tb.mu.Lock()
	defer tb.mu.Unlock()
	return tb.metrics
}
