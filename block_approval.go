package main

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"fmt"
	"math/big"
)

// BlockApproval tracks the approval signatures for a single block.
type BlockApproval struct {
	Block      *Block
	Committee  []*Validator
	Signatures map[string][]byte
	Threshold  int
}

// NewBlockApproval creates a new tracker for a given block and committee.
func NewBlockApproval(block *Block, committee []*Validator) *BlockApproval {
	return &BlockApproval{
		Block:      block,
		Committee:  committee,
		Signatures: make(map[string][]byte),
		Threshold:  (2 * len(committee) / 3) + 1,
	}
}

// AddSignature adds a signature from a validator.
func (ba *BlockApproval) AddSignature(addr Address, signature []byte) error {
	if !ba.isCommitteeMember(addr) {
		return fmt.Errorf("address %s is not in the committee", addr.ToHex())
	}
	if ba.HasSignature(addr) {
		return fmt.Errorf("already have signature from %s", addr.ToHex())
	}
	ba.Signatures[addr.ToHex()] = signature
	return nil
}

// HasSignature checks if a validator has already signed.
func (ba *BlockApproval) HasSignature(validator Address) bool {
	_, ok := ba.Signatures[validator.ToHex()]
	return ok
}

// IsApproved returns true if at least 2/3 of committee have signed.
func (ba *BlockApproval) IsApproved() bool {
	return len(ba.Signatures) >= ba.Threshold
}

// isCommitteeMember checks if an address is in the committee.
func (ba *BlockApproval) isCommitteeMember(addr Address) bool {
	for _, member := range ba.Committee {
		if member.Address == addr {
			return true
		}
	}
	return false
}

// VerifySignature verifies a signature for a given validator and block hash.
func VerifySignature(pubKey *ecdsa.PublicKey, hash Hash, sig []byte) bool {
	if len(sig) < 64 {
		return false
	}
	r := new(big.Int).SetBytes(sig[:len(sig)/2])
	s := new(big.Int).SetBytes(sig[len(sig)/2:])
	return ecdsa.Verify(pubKey, hash[:], r, s)
}

// HashBlockForSign returns the hash to be signed for a block.
func HashBlockForSign(block *Block) Hash {
	// For now, just use SHA256 of the block header hash
	return sha256.Sum256(block.Header.Hash[:])
}
