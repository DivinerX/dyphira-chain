package main

import (
	"bytes"
	"fmt"

	"golang.org/x/crypto/sha3"
)

// Node represents a node in the Merkle Trie.
type Node struct {
	Hash  Hash
	Value []byte
	Key   []byte // Store the original key
	Left  *Node
	Right *Node
}

// MerkleTrie represents the Merkle Trie structure.
type MerkleTrie struct {
	Root *Node
	// Keep a map for efficient retrieval of all key-value pairs
	kvMap map[string][]byte
}

// NewMerkleTrie creates a new Merkle Trie.
func NewMerkleTrie() *MerkleTrie {
	return &MerkleTrie{
		Root:  &Node{},
		kvMap: make(map[string][]byte),
	}
}

// Insert adds a key-value pair to the trie.
func (t *MerkleTrie) Insert(key []byte, value []byte) {
	t.Root = t.insert(t.Root, key, value, 0)
	// Store in map for efficient retrieval
	t.kvMap[string(key)] = value
}

func (t *MerkleTrie) insert(node *Node, key []byte, value []byte, depth int) *Node {
	if node == nil {
		node = &Node{}
	}

	if depth == len(key)*8 {
		node.Value = value
		node.Key = key // Store the original key
		node.Hash = Hash(sha3.Sum256(value))
		return node
	}

	bit := (key[depth/8] >> (7 - (depth % 8))) & 1
	if bit == 0 {
		node.Left = t.insert(node.Left, key, value, depth+1)
	} else {
		node.Right = t.insert(node.Right, key, value, depth+1)
	}

	node.Hash = t.recalculateHash(node)
	return node
}

// Get retrieves a value by its key.
func (t *MerkleTrie) Get(key []byte) ([]byte, bool) {
	node := t.get(t.Root, key, 0)
	if node != nil && node.Value != nil {
		return node.Value, true
	}
	return nil, false
}

func (t *MerkleTrie) get(node *Node, key []byte, depth int) *Node {
	if node == nil {
		return nil
	}
	if depth == len(key)*8 {
		return node
	}

	bit := (key[depth/8] >> (7 - (depth % 8))) & 1
	if bit == 0 {
		return t.get(node.Left, key, depth+1)
	}
	return t.get(node.Right, key, depth+1)
}

func (t *MerkleTrie) recalculateHash(node *Node) Hash {
	var leftHash, rightHash [32]byte
	if node.Left != nil {
		leftHash = node.Left.Hash
	}
	if node.Right != nil {
		rightHash = node.Right.Hash
	}

	combined := append(leftHash[:], rightHash[:]...)
	hash := sha3.Sum256(combined)
	return Hash(hash)
}

func (t *MerkleTrie) String() string {
	var buf bytes.Buffer
	t.print(t.Root, 0, &buf)
	return buf.String()
}

func (t *MerkleTrie) print(node *Node, indent int, buf *bytes.Buffer) {
	if node == nil {
		return
	}
	fmt.Fprintf(buf, "%s- Hash: %s, Value: %s\n", bytes.Repeat([]byte("  "), indent), node.Hash.ToHex(), string(node.Value))
	t.print(node.Left, indent+1, buf)
	t.print(node.Right, indent+1, buf)
}

// KVPair represents a key-value pair in the trie
type KVPair struct {
	Key   []byte
	Value []byte
}

// All returns all key-value pairs in the trie
func (t *MerkleTrie) All() []KVPair {
	var pairs []KVPair
	for key, value := range t.kvMap {
		pairs = append(pairs, KVPair{
			Key:   []byte(key),
			Value: value,
		})
	}
	return pairs
}
