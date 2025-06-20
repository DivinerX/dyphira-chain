package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeTestBlockWithHash(h byte) *Block {
	header := &Header{BlockNumber: uint64(h)}
	header.Hash = Hash{h}
	return &Block{Header: header}
}

func TestForkChoiceSelectHead(t *testing.T) {
	store := NewMemoryStore()
	vr := NewValidatorRegistry(store, "validators")
	pt := NewParticipationTracker()
	chain, _ := NewBlockchain(store)

	// Register validators
	v1 := &Validator{Address: Address{1}, Stake: 100}
	v2 := &Validator{Address: Address{2}, Stake: 50}
	v3 := &Validator{Address: Address{3}, Stake: 25}
	_ = vr.RegisterValidator(v1)
	_ = vr.RegisterValidator(v2)
	_ = vr.RegisterValidator(v3)

	// Create blocks
	b1 := makeTestBlockWithHash(1)
	b2 := makeTestBlockWithHash(2)
	b3 := makeTestBlockWithHash(3)

	// Simulate votes
	pt.RecordVote(b1.Header.Hash, v1) // 100
	pt.RecordVote(b2.Header.Hash, v2) // 50
	pt.RecordVote(b2.Header.Hash, v3) // 25

	fc := &ForkChoice{Tracker: pt, Registry: vr, Chain: chain}
	head, err := fc.SelectHead()
	assert.Nil(t, err)
	// b2 should win (50+25=75 < 100, so b1 wins)
	assert.Equal(t, b1.Header.Hash, head)

	// Now v2 and v3 both vote for b2 (total 75), v1 for b1 (100)
	// If v1 switches to b2, b2 wins
	pt.RecordVote(b2.Header.Hash, v1)
	head, err = fc.SelectHead()
	assert.Nil(t, err)
	assert.Equal(t, b2.Header.Hash, head)

	// If all vote for b3, b3 wins
	pt.RecordVote(b3.Header.Hash, v1)
	pt.RecordVote(b3.Header.Hash, v2)
	pt.RecordVote(b3.Header.Hash, v3)
	head, err = fc.SelectHead()
	assert.Nil(t, err)
	assert.Equal(t, b3.Header.Hash, head)
}
