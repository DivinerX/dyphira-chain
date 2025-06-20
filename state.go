package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"errors"
	"fmt"
)

type State struct {
	Trie *MerkleTrie
}

func NewState() *State {
	return &State{Trie: NewMerkleTrie()}
}

// GetAccount retrieves an account from the trie.
func (s *State) GetAccount(addr Address) (*Account, error) {
	data, found := s.Trie.Get(addr[:])
	if !found {
		return nil, errors.New("account not found")
	}
	var acc Account
	if err := json.Unmarshal(data, &acc); err != nil {
		return nil, err
	}
	return &acc, nil
}

// PutAccount stores an account in the trie.
func (s *State) PutAccount(acc *Account) error {
	data, err := json.Marshal(acc)
	if err != nil {
		return err
	}
	s.Trie.Insert(acc.Address[:], data)
	return nil
}

// ApplyTransaction applies a transaction to the state.
func (s *State) ApplyTransaction(tx *Transaction) error {
	sender, err := s.GetAccount(tx.From)
	if err != nil {
		return err
	}

	if sender.Nonce+1 != tx.Nonce {
		return fmt.Errorf("invalid nonce. got %d, want %d", tx.Nonce, sender.Nonce+1)
	}

	if tx.Value > sender.Balance {
		return errors.New("insufficient balance")
	}

	// Participation transaction logic
	if tx.Type == "participation" {
		// Mark validator as participating
		if s.Trie != nil {
			// We'll update the ValidatorRegistry in the block application logic
		}
	}

	sender.Balance -= tx.Value
	sender.Nonce += 1

	toAcc, _ := s.GetAccount(tx.To)
	if toAcc == nil {
		// If the recipient doesn't exist, create a new account.
		toAcc = &Account{Address: tx.To, Balance: 0, Nonce: 0}
	}
	toAcc.Balance += tx.Value

	if err := s.PutAccount(sender); err != nil {
		return err
	}
	if err := s.PutAccount(toAcc); err != nil {
		return err
	}
	return nil
}

// ApplyBlock applies all transactions in a block to the state.
func (s *State) ApplyBlock(block *Block) error {
	for _, tx := range block.Transactions {
		if err := s.ApplyTransaction(tx); err != nil {
			// In a real implementation, we would need to handle this failure
			// gracefully, potentially rolling back the state changes.
			// For now, we'll just return the error.
			return fmt.Errorf("failed to apply transaction %s: %w", tx.Hash, err)
		}
	}
	return nil
}

// Helper: derive address from public key (simple hash for demo)
func pubKeyToAddress(pub *ecdsa.PublicKey) Address {
	b := elliptic.Marshal(pub.Curve, pub.X, pub.Y)
	var addr Address
	// Use last 20 bytes of the public key as the address (similar to Ethereum)
	copy(addr[:], b[len(b)-20:])
	return addr
}
