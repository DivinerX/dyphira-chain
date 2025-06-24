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

	txs := make([]*Transaction, 0, n)
	for _, tx := range tp.transactions {
		if len(txs) >= n {
			break
		}

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
		log.Printf("DEBUG: Selecting transaction %s (To: %s, Value: %d, Fee: %d)",
			tx.Hash.ToHex(), tx.To.ToHex(), tx.Value, tx.Fee)
		txs = append(txs, tx)
	}

	log.Printf("DEBUG: SelectTransactions returning %d transactions", len(txs))
	return txs
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
