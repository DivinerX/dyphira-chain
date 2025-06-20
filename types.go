package main

import (
	"crypto/ecdsa"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"

	"golang.org/x/crypto/sha3"
)

func init() {
	gob.Register(&Block{})
	gob.Register(&Header{})
	gob.Register(&Transaction{})
}

// Hash represents the 32-byte SHA256 hash of arbitrary data.
type Hash [32]byte

// ToHex returns the hex representation of the hash.
func (h Hash) ToHex() string {
	return hex.EncodeToString(h[:])
}

// Address represents the 20-byte address of an account.
type Address [20]byte

// ToHex converts an Address to its hexadecimal representation.
func (a Address) ToHex() string {
	return hex.EncodeToString(a[:])
}

// PublicKey is the public key of a validator or user.
type PublicKey ecdsa.PublicKey

// Signature is a digital signature.
type Signature []byte

// Block represents a single block in the blockchain.
type Block struct {
	Header        *Header
	Transactions  []*Transaction
	ValidatorList []*Validator
	Signature     []byte // Proposer's signature on the block header hash
	Size          uint64 `json:"size"` // The overall size in bytes of the block
}

// Header represents the header of a block.
type Header struct {
	BlockNumber     uint64  `json:"blockNumber"`
	PreviousHash    Hash    `json:"previousHash"`
	Timestamp       int64   `json:"timestamp"`
	Proposer        Address `json:"proposer"`
	Gas             uint64  `json:"gas"` // The total amount of transaction fees for the current block
	TransactionRoot Hash    `json:"transactionRoot"`
	Hash            Hash    `json:"hash"` // Hash of the current block header
}

// Transaction represents a single transaction.
type Transaction struct {
	From      Address `json:"from"`
	To        Address `json:"to"`
	Value     uint64  `json:"value"`
	Nonce     uint64  `json:"nonce"`
	Fee       uint64  `json:"fee"`
	Timestamp int64   `json:"timestamp"`
	Type      string  `json:"type"` // "transfer" or "participation"
	R, S      []byte  `json:"-"`    // Signature components, ignored by json
	Hash      Hash    `json:"hash"`
}

// Encode serializes the Transaction to a JSON byte slice for hashing.
func (t *Transaction) Encode() ([]byte, error) {
	// Create a temporary tx without signature components for hashing
	tempTx := *t
	tempTx.R, tempTx.S, tempTx.Hash = nil, nil, Hash{}
	return json.Marshal(tempTx)
}

// Decode deserializes a JSON byte slice into a Transaction.
func (t *Transaction) Decode(data []byte) error {
	return json.Unmarshal(data, t)
}

// NetworkTransaction is a wrapper for broadcasting a transaction with its public key.
type NetworkTransaction struct {
	Tx     *Transaction     `json:"tx"`
	PubKey *ecdsa.PublicKey `json:"pubKey"`
}

// Encode serializes the NetworkTransaction to a JSON byte slice.
func (nt *NetworkTransaction) Encode() ([]byte, error) {
	return json.Marshal(nt)
}

// Decode deserializes a JSON byte slice into a NetworkTransaction.
func (nt *NetworkTransaction) Decode(data []byte) error {
	return json.Unmarshal(data, nt)
}

// Account represents a user account.
type Account struct {
	Address Address `json:"address"`
	Balance uint64  `json:"balance"`
	Nonce   uint64  `json:"nonce"`
}

// Validator represents a validator node.
type Validator struct {
	Address           Address `json:"address"`
	Stake             uint64  `json:"stake"`
	DelegatedStake    uint64  `json:"delegatedStake"`
	ComputeReputation uint64  `json:"computeReputation"`
	Participating     bool    `json:"participating"`
}

// Helper functions for testing and setup
func NewBlock(header *Header, txs []*Transaction) *Block {
	return &Block{
		Header:       header,
		Transactions: txs,
	}
}

// Encode serializes the Block to a JSON byte slice.
func (b *Block) Encode() ([]byte, error) {
	return json.Marshal(b)
}

// Decode deserializes a JSON byte slice into a Block.
func (b *Block) Decode(data []byte) error {
	return json.Unmarshal(data, b)
}

// ComputeHash calculates the hash of the block header.
func (h *Header) ComputeHash() (Hash, error) {
	// Create a temporary header without the hash itself to ensure consistent hashing
	tempHeader := *h
	tempHeader.Hash = Hash{}

	b, err := json.Marshal(tempHeader)
	if err != nil {
		return Hash{}, err
	}
	hash := sha3.Sum256(b)
	return Hash(hash), nil
}

// Approval represents a validator's signature for a specific block.
type Approval struct {
	BlockHash Hash    `json:"blockHash"`
	Address   Address `json:"address"`
	Signature []byte  `json:"signature"`
}
