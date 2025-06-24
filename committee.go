package main

import (
	"bytes"
	"math/rand"
	"sort"
)

// CommitteeSelector handles committee and proposer selection.
type CommitteeSelector struct {
	Registry *ValidatorRegistry
}

// CommitteeMember is a validator with its selection weight.
type CommitteeMember struct {
	Validator *Validator
	Weight    uint64
}

// SelectCommittee selects up to N validators for the next epoch, weighted by stake, delegation, and reputation.
func (cs *CommitteeSelector) SelectCommittee(n int) ([]*Validator, error) {
	validators, err := cs.Registry.GetAllValidators()
	if err != nil {
		return nil, err
	}
	if len(validators) == 0 {
		return nil, nil
	}

	// Only consider participating validators
	participating := make([]*Validator, 0, len(validators))
	for _, v := range validators {
		if v.Participating {
			participating = append(participating, v)
		}
	}
	if len(participating) == 0 {
		return nil, nil
	}

	// Calculate weights
	members := make([]CommitteeMember, 0, len(participating))
	for _, v := range participating {
		weight := v.Stake + v.DelegatedStake + v.ComputeReputation
		members = append(members, CommitteeMember{Validator: v, Weight: weight})
	}

	// Sort by weight descending, then by address ascending for determinism
	sort.Slice(members, func(i, j int) bool {
		if members[i].Weight == members[j].Weight {
			return bytes.Compare(members[i].Validator.Address[:], members[j].Validator.Address[:]) < 0
		}
		return members[i].Weight > members[j].Weight
	})

	// Select top N (or all if fewer)
	committee := make([]*Validator, 0, n)
	for i := 0; i < n && i < len(members); i++ {
		committee = append(committee, members[i].Validator)
	}
	return committee, nil
}

// ProposerSelector manages leader rotation for block production.
type ProposerSelector struct {
	Committee   []*Validator
	Slots       map[uint64]*Validator // blockHeight -> proposer
	EpochStart  uint64
	EpochLength uint64
}

// NewProposerSelectorWithRotation creates a proposer selector with 9-block slots per validator, shuffled randomly.
func NewProposerSelectorWithRotation(committee []*Validator, epochStart, epochLength uint64) *ProposerSelector {
	slots := make(map[uint64]*Validator)
	if len(committee) == 0 || epochLength == 0 {
		return &ProposerSelector{Committee: committee, Slots: slots, EpochStart: epochStart, EpochLength: epochLength}
	}

	// Shuffle committee for random order
	r := rand.New(rand.NewSource(0))
	indices := r.Perm(len(committee))

	// Each validator gets exactly 9 consecutive blocks
	blocksPerValidator := uint64(9)
	block := epochStart

	for _, idx := range indices {
		validator := committee[idx]
		for i := uint64(0); i < blocksPerValidator && block < epochStart+epochLength; i++ {
			slots[block] = validator
			block++
		}
	}

	// If there are leftover blocks (epochLength not divisible by 9*len(committee)), assign round-robin
	for block < epochStart+epochLength {
		validator := committee[(int(block-epochStart))%len(committee)]
		slots[block] = validator
		block++
	}

	return &ProposerSelector{Committee: committee, Slots: slots, EpochStart: epochStart, EpochLength: epochLength}
}

// ProposerForBlock returns the proposer for a given block height.
func (ps *ProposerSelector) ProposerForBlock(blockHeight uint64) *Validator {
	return ps.Slots[blockHeight]
}
