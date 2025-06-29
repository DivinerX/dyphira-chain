package main

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcutil/bech32"
	"github.com/jzelinskie/whirlpool"
	"golang.org/x/crypto/ripemd160"
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
		// Return a new account with 0 balance and nonce 0
		return &Account{Address: addr, Balance: 0, Nonce: 0}, nil
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

	if tx.Value+tx.Fee > sender.Balance {
		return errors.New("insufficient balance")
	}

	// Handle different transaction types
	switch tx.Type {
	case "participation":
		// Participation transaction logic - handled in block application
		if s.Trie != nil {
			// We'll update the ValidatorRegistry in the block application logic
		}
	case "register_validator":
		// Validator registration - stake is locked from sender's balance
		// The actual registration happens in block application
		if tx.Value == 0 {
			return errors.New("validator registration requires non-zero stake")
		}
	case "delegate":
		// Delegation - stake is transferred from delegator to validator
		// The actual delegation happens in block application
		if tx.Value == 0 {
			return errors.New("delegation requires non-zero amount")
		}
	case "transfer":
		// Standard transfer - handled below
	default:
		return fmt.Errorf("unknown transaction type: %s", tx.Type)
	}

	sender.Balance -= (tx.Value + tx.Fee)
	sender.Nonce += 1

	// For transfer transactions, update recipient balance
	if tx.Type == "transfer" {
		toAcc, _ := s.GetAccount(tx.To)
		if toAcc == nil {
			// If the recipient doesn't exist, create a new account.
			toAcc = &Account{Address: tx.To, Balance: 0, Nonce: 0}
		}
		toAcc.Balance += tx.Value

		if err := s.PutAccount(toAcc); err != nil {
			return err
		}
	}

	if err := s.PutAccount(sender); err != nil {
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

// Helper: derive address from public key (Whirlpool -> RIPEMD-160, Secp256k1)
func pubKeyToAddress(pub *btcec.PublicKey) Address {
	b := pub.SerializeUncompressed()
	whirlpoolHash := whirlpool.New()
	whirlpoolHash.Write(b)
	whirlpoolDigest := whirlpoolHash.Sum(nil)
	ripemd := ripemd160.New()
	ripemd.Write(whirlpoolDigest[:32])
	ripeDigest := ripemd.Sum(nil)
	var addr Address
	copy(addr[:], ripeDigest[:20])
	return addr
}

// BECH-32 encode an Address for display (HRP = "dpos")
func AddressToBech32(addr Address) (string, error) {
	fiveBit, err := bech32.ConvertBits(addr[:], 8, 5, true)
	if err != nil {
		return "", err
	}
	return bech32.Encode("dpos", fiveBit)
}

// Helper: generate a new Secp256k1 private key
func GenerateSecp256k1Key() (*btcec.PrivateKey, error) {
	return btcec.NewPrivateKey()
}

// ExportSnapshot exports all accounts in the state as a snapshot
func (s *State) ExportSnapshot() ([]*Account, error) {
	accounts := []*Account{}
	if s.Trie == nil {
		return accounts, nil
	}
	for _, kv := range s.Trie.All() {
		var acc Account
		if err := json.Unmarshal(kv.Value, &acc); err != nil {
			return nil, err
		}
		accounts = append(accounts, &acc)
	}
	return accounts, nil
}

// ImportSnapshot imports a snapshot of accounts into the state (overwrites existing)
func (s *State) ImportSnapshot(accounts []*Account) error {
	if s.Trie == nil {
		s.Trie = NewMerkleTrie()
	}
	for _, acc := range accounts {
		if err := s.PutAccount(acc); err != nil {
			return err
		}
	}
	return nil
}
