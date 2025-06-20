package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func makeTestValidators(vr *ValidatorRegistry, n int) []Address {
	addrs := make([]Address, n)
	for i := 0; i < n; i++ {
		addr := Address{byte(i + 1)}
		v := &Validator{
			Address:           addr,
			Stake:             uint64(100 + i*10),
			DelegatedStake:    uint64(i * 5),
			ComputeReputation: uint64(i),
			Participating:     true,
		}
		_ = vr.RegisterValidator(v)
		addrs[i] = addr
	}
	return addrs
}

func TestCommitteeSelection(t *testing.T) {
	store := NewMemoryStore()
	vr := NewValidatorRegistry(store, "validators")
	_ = makeTestValidators(vr, 10)
	cs := &CommitteeSelector{Registry: vr}

	committee, err := cs.SelectCommittee(5)
	assert.Nil(t, err)
	assert.Equal(t, 5, len(committee))

	// Should be sorted by weight descending
	for i := 1; i < len(committee); i++ {
		prev := committee[i-1].Stake + committee[i-1].DelegatedStake + committee[i-1].ComputeReputation
		curr := committee[i].Stake + committee[i].DelegatedStake + committee[i].ComputeReputation
		assert.True(t, prev >= curr)
	}
}

func TestProposerSelector(t *testing.T) {
	store := NewMemoryStore()
	vr := NewValidatorRegistry(store, "validators")
	_ = makeTestValidators(vr, 3)
	cs := &CommitteeSelector{Registry: vr}
	committee, _ := cs.SelectCommittee(3)
	ps := NewProposerSelectorWithRotation(committee, 0, 27)

	order := []Address{}
	for i := 0; i < 27; i++ {
		v := ps.ProposerForBlock(uint64(i))
		order = append(order, v.Address)
	}
	// Each validator should have 9 consecutive slots (in some order)
	counts := make(map[Address]int)
	for _, addr := range order {
		counts[addr]++
	}
	for _, v := range committee {
		assert.Equal(t, 9, counts[v.Address])
	}
}

func TestCommitteeSelection_Participation(t *testing.T) {
	store := NewMemoryStore()
	vr := NewValidatorRegistry(store, "validators")
	addrs := make([]Address, 3)
	for i := 0; i < 3; i++ {
		addr := Address{byte(i + 1)}
		v := &Validator{Address: addr, Stake: 100}
		if i == 0 {
			v.Participating = true // Only first validator participates
		}
		_ = vr.RegisterValidator(v)
		addrs[i] = addr
	}
	cs := &CommitteeSelector{Registry: vr}

	committee, err := cs.SelectCommittee(3)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(committee))
	assert.Equal(t, addrs[0], committee[0].Address)
}
