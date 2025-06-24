package main

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/crypto/sha3"
)

// Blockchain represents the blockchain itself.
type Blockchain struct {
	store         Storage
	lock          sync.RWMutex
	currentHeight uint64
	tip           Hash
	height        uint64
	genesisBlock  *Block
}

// In Blockchain struct, add a constant for the height key
const heightKey = "height"

// NewBlockchain creates a new blockchain with a genesis block.
func NewBlockchain(store Storage) (*Blockchain, error) {
	genesis := createGenesisBlock()
	bc := &Blockchain{
		store:         store,
		genesisBlock:  genesis,
		currentHeight: 0, // Start with height 0 (genesis)
	}

	// Try to load existing tip
	tipData, err := store.Get([]byte("tip"))
	if err == nil && len(tipData) == 32 {
		var tip Hash
		copy(tip[:], tipData)
		bc.tip = tip

		// Load the current height directly
		heightData, err := store.Get([]byte(heightKey))
		if err == nil && len(heightData) > 0 {
			var h uint64
			if err := json.Unmarshal(heightData, &h); err == nil {
				bc.currentHeight = h
			}
		}
	} else {
		// If no tip, it's a fresh chain, tip is genesis hash
		bc.tip = genesis.Header.Hash
		bc.currentHeight = 0
		if err := store.Put([]byte("tip"), bc.tip[:]); err != nil {
			return nil, fmt.Errorf("failed to save initial tip: %w", err)
		}
		if err := store.Put([]byte(heightKey), mustMarshalUint64(0)); err != nil {
			return nil, fmt.Errorf("failed to save initial height: %w", err)
		}
		// Save genesis block
		if err := bc.addBlock(genesis); err != nil {
			return nil, fmt.Errorf("failed to save genesis block: %w", err)
		}
	}

	return bc, nil
}

// GetLastBlock returns the most recent block in the chain.
func (bc *Blockchain) GetLastBlock() (*Block, error) {
	bc.lock.RLock()
	defer bc.lock.RUnlock()
	if bc.tip == (Hash{}) {
		// This can happen if the database is fresh, return genesis.
		return bc.genesisBlock, nil
	}
	return bc.GetBlockByHash(bc.tip)
}

// AddBlock adds a new block to the blockchain.
func (bc *Blockchain) AddBlock(b *Block) error {
	bc.lock.Lock()
	defer bc.lock.Unlock()
	log.Printf("DEBUG: AddBlock called for block #%d (hash: %s), currentHeight before: %d", b.Header.BlockNumber, b.Header.Hash.ToHex(), bc.currentHeight)
	return bc.addBlock(b)
}

// GetBlockByHeight retrieves a block by its height.
func (bc *Blockchain) GetBlockByHeight(height uint64) (*Block, error) {
	if height > bc.currentHeight {
		return nil, fmt.Errorf("height %d is too high (current: %d)", height, bc.currentHeight)
	}

	// Handle genesis block
	if height == 0 {
		return bc.genesisBlock, nil
	}

	key := blockKey(height)
	data, err := bc.store.Get(key)
	if err != nil {
		return nil, err
	}
	return decodeBlock(data)
}

// GetBlockByHash retrieves a block by its hash.
func (bc *Blockchain) GetBlockByHash(hash Hash) (*Block, error) {
	// This is inefficient for a simple memory store, but demonstrates the concept.
	// A real implementation would have a hash-to-height index.
	for i := uint64(0); i <= bc.currentHeight; i++ {
		block, err := bc.GetBlockByHeight(i)
		if err != nil {
			continue
		}
		if block.Header.Hash == hash {
			return block, nil
		}
	}
	return nil, fmt.Errorf("block with hash %s not found", hash.ToHex())
}

// HasBlock checks if a block with the given hash exists in the blockchain.
func (bc *Blockchain) HasBlock(hash Hash) bool {
	// This is inefficient, like GetBlockByHash. A hash-to-height index would be better.
	_, err := bc.GetBlockByHash(hash)
	return err == nil
}

// Height returns the current height of the blockchain.
func (bc *Blockchain) Height() uint64 {
	bc.lock.RLock()
	defer bc.lock.RUnlock()
	log.Printf("DEBUG: Height() called, returning %d", bc.currentHeight)
	return bc.currentHeight
}

func (bc *Blockchain) addBlock(b *Block) error {
	key := blockKey(b.Header.BlockNumber)
	data, err := encodeBlock(b)
	if err != nil {
		return err
	}
	if err := bc.store.Put(key, data); err != nil {
		return err
	}
	// Also update the tip
	bc.tip = b.Header.Hash
	bc.height = b.Header.BlockNumber
	if err := bc.store.Put([]byte("tip"), bc.tip[:]); err != nil {
		return fmt.Errorf("failed to update tip: %w", err)
	}

	bc.currentHeight = b.Header.BlockNumber
	if err := bc.store.Put([]byte(heightKey), mustMarshalUint64(bc.currentHeight)); err != nil {
		return fmt.Errorf("failed to update height: %w", err)
	}
	log.Printf("DEBUG: addBlock updated currentHeight to %d", bc.currentHeight)
	return nil
}

func createGenesisBlock() *Block {
	header := &Header{
		BlockNumber: 0,
		Timestamp:   1672531200, // Jan 1, 2023
	}
	hash, _ := header.ComputeHash()
	header.Hash = hash

	return &Block{
		Header: header,
	}
}

func blockKey(height uint64) []byte {
	return []byte(fmt.Sprintf("block_%d", height))
}

func encodeBlock(b *Block) ([]byte, error) {
	return json.Marshal(b)
}

func decodeBlock(data []byte) (*Block, error) {
	var b Block
	err := json.Unmarshal(data, &b)
	return &b, err
}

func (bc *Blockchain) CreateBlock(txs []*Transaction, proposer *Validator, privKey *ecdsa.PrivateKey) (*Block, error) {
	lastBlock, err := bc.GetLastBlock()
	if err != nil {
		return nil, fmt.Errorf("failed to get last block: %w", err)
	}

	header := &Header{
		BlockNumber:     lastBlock.Header.BlockNumber + 1,
		PreviousHash:    lastBlock.Header.Hash,
		Timestamp:       time.Now().UnixNano(),
		Proposer:        proposer.Address,
		TransactionRoot: computeTransactionRoot(txs),
	}

	hash, err := header.ComputeHash()
	if err != nil {
		return nil, fmt.Errorf("failed to compute header hash: %w", err)
	}
	header.Hash = hash

	block := &Block{
		Header:       header,
		Transactions: txs,
	}

	// Enforce 256MB block size limit
	blockBytes, err := encodeBlock(block)
	if err != nil {
		return nil, err
	}
	block.Size = uint64(len(blockBytes))
	if block.Size > 256*1024*1024 {
		return nil, fmt.Errorf("block size %d exceeds 256MB limit", block.Size)
	}

	return block, nil
}

// computeTransactionRoot calculates the Merkle root of a list of transactions.
func computeTransactionRoot(txs []*Transaction) Hash {
	if len(txs) == 0 {
		return Hash{}
	}
	// Simple approach: hash the concatenation of all transaction hashes.
	var combinedHashes []byte
	for _, tx := range txs {
		combinedHashes = append(combinedHashes, tx.Hash[:]...)
	}
	hash := sha3.Sum256(combinedHashes)
	return Hash(hash)
}

// Close closes the blockchain's underlying database.
func (bc *Blockchain) Close() error {
	// The store is managed by the AppNode, which is responsible for closing it.
	// This method is here to satisfy any interfaces that might expect it, but it's a no-op.
	return nil
}

// ApplyBlockWithRegistry applies a block's transactions to the state and updates the validator registry.
func (bc *Blockchain) ApplyBlockWithRegistry(block *Block, state *State, vr *ValidatorRegistry) error {
	for _, tx := range block.Transactions {
		// Handle special transaction types that affect the validator registry
		switch tx.Type {
		case "participation":
			v, err := vr.GetValidator(tx.From)
			if err == nil && v != nil && !v.Participating {
				v.Participating = true
				_ = vr.RegisterValidator(v)
			}
		case "register_validator":
			// Register or update validator with the staked amount
			v, err := vr.GetValidator(tx.From)
			if err != nil {
				// Create new validator
				v = &Validator{
					Address:           tx.From,
					Stake:             tx.Value,
					DelegatedStake:    0,
					ComputeReputation: 0,
					Participating:     true, // Auto-participate when registered
				}
			} else if v == nil {
				// Validator not found, create new one
				v = &Validator{
					Address:           tx.From,
					Stake:             tx.Value,
					DelegatedStake:    0,
					ComputeReputation: 0,
					Participating:     true, // Auto-participate when registered
				}
			} else {
				// Update existing validator's stake
				v.Stake += tx.Value
				v.Participating = true
			}
			_ = vr.RegisterValidator(v)
		case "delegate":
			// Delegate stake from sender to recipient (validator)
			v, err := vr.GetValidator(tx.To)
			if err != nil {
				return fmt.Errorf("cannot delegate to non-registered validator %s: %w", tx.To.ToHex(), err)
			}
			if v == nil {
				return fmt.Errorf("validator %s not found", tx.To.ToHex())
			}
			// Add delegated stake to the validator
			v.DelegatedStake += tx.Value
			_ = vr.RegisterValidator(v)
		}

		// Apply the transaction to state (balance updates, nonce increments, etc.)
		if err := state.ApplyTransaction(tx); err != nil {
			return fmt.Errorf("failed to apply transaction %s: %w", tx.Hash, err)
		}
	}
	return nil
}

// Helper to marshal uint64
func mustMarshalUint64(h uint64) []byte {
	b, _ := json.Marshal(h)
	return b
}
