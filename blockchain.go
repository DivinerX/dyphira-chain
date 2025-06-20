package main

import (
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"golang.org/x/crypto/sha3"
)

// IBlockStore defines the interface for a key-value store for the blockchain.
type IBlockStore interface {
	Put(key, value []byte) error
	Get(key []byte) ([]byte, error)
	Close() error
}

// Blockchain represents the blockchain itself.
type Blockchain struct {
	store         IBlockStore
	lock          sync.RWMutex
	currentHeight uint64
	tip           Hash
	height        uint64
}

// NewBlockchain creates a new blockchain with a genesis block.
func NewBlockchain(store IBlockStore) (*Blockchain, error) {
	bc := &Blockchain{
		store:         store,
		currentHeight: 0,
	}

	genesis := createGenesisBlock()
	err := bc.addBlock(genesis)
	if err != nil {
		return nil, err
	}
	return bc, nil
}

// AddBlock adds a new block to the blockchain.
func (bc *Blockchain) AddBlock(b *Block) error {
	bc.lock.Lock()
	defer bc.lock.Unlock()
	return bc.addBlock(b)
}

// GetBlockByHeight retrieves a block by its height.
func (bc *Blockchain) GetBlockByHeight(height uint64) (*Block, error) {
	if height > bc.currentHeight {
		return nil, fmt.Errorf("height %d is too high", height)
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

// Height returns the current height of the blockchain.
func (bc *Blockchain) Height() uint64 {
	bc.lock.RLock()
	defer bc.lock.RUnlock()
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
	bc.currentHeight = b.Header.BlockNumber
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
	lastBlock, err := bc.GetBlockByHeight(bc.Height())
	if err != nil {
		// Handle genesis block case
		if bc.Height() == 0 {
			lastBlock = &Block{Header: &Header{BlockNumber: 0, Hash: Hash{}}}
		} else {
			return nil, fmt.Errorf("failed to get last block: %w", err)
		}
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
	return bc.store.Close()
}

func (bc *Blockchain) ApplyBlockWithRegistry(block *Block, state *State, vr *ValidatorRegistry) error {
	for _, tx := range block.Transactions {
		if tx.Type == "participation" {
			v, err := vr.GetValidator(tx.From)
			if err == nil && v != nil && !v.Participating {
				v.Participating = true
				_ = vr.RegisterValidator(v)
			}
		}
		if err := state.ApplyTransaction(tx); err != nil {
			return fmt.Errorf("failed to apply transaction %s: %w", tx.Hash, err)
		}
	}
	return nil
}
