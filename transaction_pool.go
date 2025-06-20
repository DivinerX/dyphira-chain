package main

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"sync"
)

type TransactionPool struct {
	mu           sync.RWMutex
	transactions map[Hash]*Transaction
}

func NewTransactionPool() *TransactionPool {
	return &TransactionPool{
		transactions: make(map[Hash]*Transaction),
	}
}

// AddTransaction validates and adds a transaction to the pool.
func (tp *TransactionPool) AddTransaction(tx *Transaction, pubKey *ecdsa.PublicKey, state *State) error {
	tp.mu.Lock()
	defer tp.mu.Unlock()

	// 1. Verify Signature
	if !VerifySignature(pubKey, tx.Hash, append(tx.R, tx.S...)) {
		return errors.New("invalid signature")
	}

	if tx.Type == "participation" {
		// Only check for duplicates
		if _, ok := tp.transactions[tx.Hash]; ok {
			return errors.New("transaction already in pool")
		}
		tp.transactions[tx.Hash] = tx
		log.Printf("Added participation transaction to pool: %x", tx.Hash)
		return nil
	}

	// 2. Check nonce and balance from state
	senderAddr := pubKeyToAddress(pubKey)
	sender, err := state.GetAccount(senderAddr)
	if err != nil {
		// Note: We might allow txs from new accounts if they are funded by a genesis block, for example
		return fmt.Errorf("could not get sender account: %w", err)
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

// RemoveTransaction removes a transaction from the pool by hash.
func (tp *TransactionPool) RemoveTransaction(hash [32]byte) {
	tp.mu.Lock()
	defer tp.mu.Unlock()
	delete(tp.transactions, hash)
}

// SelectTransactions selects up to n transactions for block inclusion.
func (tp *TransactionPool) SelectTransactions(n int) []*Transaction {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	txs := make([]*Transaction, 0, n)
	for _, tx := range tp.transactions {
		if len(txs) >= n {
			break
		}
		txs = append(txs, tx)
	}
	return txs
}

// Size returns the number of transactions in the pool.
func (tp *TransactionPool) Size() int {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	return len(tp.transactions)
}
