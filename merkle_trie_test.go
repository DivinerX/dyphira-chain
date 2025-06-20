package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMerkleTrie(t *testing.T) {
	trie := NewMerkleTrie()

	// Test Insert and Get
	key1 := []byte("hello")
	value1 := []byte("world")
	trie.Insert(key1, value1)

	retrievedValue, found := trie.Get(key1)
	assert.True(t, found)
	assert.Equal(t, value1, retrievedValue)

	// Test with another key
	key2 := []byte("foo")
	value2 := []byte("bar")
	trie.Insert(key2, value2)

	retrievedValue, found = trie.Get(key2)
	assert.True(t, found)
	assert.Equal(t, value2, retrievedValue)

	// Test non-existent key
	retrievedValue, found = trie.Get([]byte("nonexistent"))
	assert.False(t, found)
	assert.Nil(t, retrievedValue)

	// Test root hash changes
	initialRootHash := trie.Root.Hash
	key3 := []byte("another")
	value3 := []byte("entry")
	trie.Insert(key3, value3)
	newRootHash := trie.Root.Hash
	assert.NotEqual(t, initialRootHash, newRootHash)
}
