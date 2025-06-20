package main

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewBlockchain(t *testing.T) {
	store := NewMemoryStore()
	bc, err := NewBlockchain(store)
	assert.Nil(t, err)
	assert.NotNil(t, bc)
	assert.Equal(t, uint64(0), bc.Height())

	genesis, err := bc.GetBlockByHeight(0)
	assert.Nil(t, err)
	assert.NotNil(t, genesis)
}

func TestAddBlock(t *testing.T) {
	store := NewMemoryStore()
	bc, _ := NewBlockchain(store)

	// Create and add a new block
	lastBlock, err := bc.GetBlockByHeight(bc.Height())
	assert.Nil(t, err)

	newBlock := &Block{
		Header: &Header{
			BlockNumber:  lastBlock.Header.BlockNumber + 1,
			PreviousHash: lastBlock.Header.Hash,
			Timestamp:    time.Now().UnixNano(),
		},
	}
	hash, err := newBlock.Header.ComputeHash()
	assert.Nil(t, err)
	newBlock.Header.Hash = hash

	err = bc.AddBlock(newBlock)
	assert.Nil(t, err)
	assert.Equal(t, uint64(1), bc.Height())

	retrievedBlock, err := bc.GetBlockByHeight(1)
	assert.Nil(t, err)
	assert.Equal(t, newBlock.Header.Hash, retrievedBlock.Header.Hash)
}

// func TestBlockSizeLimit(t *testing.T) {
// 	store := NewMemoryStore()
// 	bc, _ := NewBlockchain(store)
// 	proposer := &Validator{Address: Address{1}}

// 	// Each transaction is small, so add enough to exceed 256MB
// 	numTx := (256*1024*1024)/128 + 1000 // 128 bytes per tx (estimate), add extra to ensure over limit
// 	txs := make([]*Transaction, numTx)
// 	for i := 0; i < numTx; i++ {
// 		tx := &Transaction{
// 			From:      Address{1},
// 			To:        Address{2},
// 			Value:     1,
// 			Nonce:     uint64(i + 1),
// 			Fee:       0,
// 			Timestamp: time.Now().UnixNano(),
// 		}
// 		data, _ := tx.Encode()
// 		tx.Hash = sha3.Sum256(data)
// 		txs[i] = tx
// 	}

// 	block, err := bc.CreateBlock(txs, proposer, nil)
// 	assert.Nil(t, block)
// 	assert.Error(t, err)
// 	assert.Contains(t, err.Error(), "256MB limit")
// }
