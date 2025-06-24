package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"math/big"

	"bytes"

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

// String returns the hex representation of the hash.
func (h Hash) String() string {
	return h.ToHex()
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
	Type      string  `json:"type"` // "transfer", "participation", "register_validator", "delegate"
	R         []byte  `json:"r"`    // Signature R component
	S         []byte  `json:"s"`    // Signature S component
	Hash      Hash    `json:"hash"`
	Used      bool    `json:"used"` // Flag to prevent duplicate inclusion
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

// Sign calculates the transaction hash and signs it with the provided private key.
func (t *Transaction) Sign(privKey *ecdsa.PrivateKey) error {
	// Encode the transaction to get the data for hashing
	data, err := t.Encode()
	if err != nil {
		return err
	}
	t.Hash = sha3.Sum256(data)

	// Sign the hash
	r, s, err := ecdsa.Sign(rand.Reader, privKey, t.Hash[:])
	if err != nil {
		return err
	}

	t.R = r.Bytes()
	t.S = s.Bytes()

	return nil
}

// Verify checks the transaction signature against the given public key.
func (t *Transaction) Verify(pubKey *ecdsa.PublicKey) bool {
	if t.R == nil || t.S == nil {
		return false
	}
	r := new(big.Int).SetBytes(t.R)
	s := new(big.Int).SetBytes(t.S)

	// Re-compute the hash from the transaction data to ensure it hasn't been tampered with
	tempTx := *t
	tempTx.R, tempTx.S, tempTx.Hash = nil, nil, Hash{}
	data, err := json.Marshal(tempTx)
	if err != nil {
		log.Printf("ERROR: Failed to marshal transaction for verification: %v", err)
		return false
	}
	hash := sha3.Sum256(data)

	// Compare the re-computed hash with the one in the transaction
	if !bytes.Equal(t.Hash[:], hash[:]) {
		log.Printf("ERROR: Transaction hash mismatch during verification.")
		return false
	}

	return ecdsa.Verify(pubKey, t.Hash[:], r, s)
}

// NetworkTransaction is a wrapper for broadcasting a transaction with its public key.
type NetworkTransaction struct {
	Tx     *Transaction `json:"tx"`
	PubKey []byte       `json:"pubKey"` // Marshaled public key bytes
}

// MarshalPublicKey marshals an ecdsa.PublicKey to bytes.
func MarshalPublicKey(pub *ecdsa.PublicKey) []byte {
	return elliptic.Marshal(pub.Curve, pub.X, pub.Y)
}

// UnmarshalPublicKey unmarshals bytes to an ecdsa.PublicKey.
func UnmarshalPublicKey(curve elliptic.Curve, data []byte) (*ecdsa.PublicKey, error) {
	x, y := elliptic.Unmarshal(curve, data)
	if x == nil || y == nil {
		return nil, errors.New("invalid public key bytes")
	}
	return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
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

// ValidatorRegistration represents a validator registration message for network sharing.
type ValidatorRegistration struct {
	Address Address `json:"address"`
	Stake   uint64  `json:"stake"`
}

// Helper functions for testing and setup
func NewBlock(header *Header, txs []*Transaction) *Block {
	return &Block{
		Header:       header,
		Transactions: txs,
	}
}

// Sign the block with the proposer's private key.
func (b *Block) Sign(privKey *ecdsa.PrivateKey) error {
	// Compute the hash if not already computed
	if b.Header.Hash == (Hash{}) {
		hash, err := b.Header.ComputeHash()
		if err != nil {
			return err
		}
		b.Header.Hash = hash
	}

	// Sign the block header hash
	r, s, err := ecdsa.Sign(rand.Reader, privKey, b.Header.Hash[:])
	if err != nil {
		return err
	}
	// ecdsa.Sign returns r and s components, we need to combine them
	b.Signature = append(r.Bytes(), s.Bytes()...)
	return nil
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
