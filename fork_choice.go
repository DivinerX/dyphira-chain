package main

import (
	"sync"
)

// ParticipationTracker tracks the latest block hash each validator considers valid.
type ParticipationTracker struct {
	votes map[Address]Hash
	lock  sync.RWMutex
}

func NewParticipationTracker() *ParticipationTracker {
	return &ParticipationTracker{votes: make(map[Address]Hash)}
}

// RecordVote records a validator's latest vote (block hash).
func (pt *ParticipationTracker) RecordVote(blockHash Hash, validator *Validator) {
	pt.lock.Lock()
	defer pt.lock.Unlock()

	// Record the validator's vote for a specific block hash.
	pt.votes[validator.Address] = blockHash
}

// LatestVote returns the latest block hash a validator voted for.
func (pt *ParticipationTracker) LatestVote(validator Address) (Hash, bool) {
	pt.lock.RLock()
	defer pt.lock.RUnlock()
	h, ok := pt.votes[validator]
	return h, ok
}

// ForkChoice selects the canonical chain head based on weighted votes.
type ForkChoice struct {
	Tracker  *ParticipationTracker
	Registry *ValidatorRegistry
	Chain    *Blockchain
}

// SelectHead returns the block hash with the highest total vote weight.
func (fc *ForkChoice) SelectHead() (Hash, error) {
	// Tally votes for each block hash
	weightMap := make(map[Hash]uint64)
	validators, err := fc.Registry.ListValidators()
	if err != nil {
		return Hash{}, err
	}
	for _, v := range validators {
		vote, ok := fc.Tracker.LatestVote(v.Address)
		if !ok {
			continue
		}
		weight := v.Stake + v.DelegatedStake + v.ComputeReputation
		weightMap[vote] += weight
	}

	// Find the block hash with the highest weight
	var maxHash Hash
	var maxWeight uint64
	for h, w := range weightMap {
		if w > maxWeight {
			maxWeight = w
			maxHash = h
		}
	}
	return maxHash, nil
}
