package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatorRegistry(t *testing.T) {
	store := NewMemoryStore()
	vr := NewValidatorRegistry(store, "validators")

	addr := Address{1, 2, 3}
	v := &Validator{
		Address:           addr,
		Stake:             100,
		DelegatedStake:    50,
		ComputeReputation: 10,
	}

	// Register
	err := vr.RegisterValidator(v)
	assert.Nil(t, err)

	// Get
	v2, err := vr.GetValidator(addr)
	assert.Nil(t, err)
	assert.Equal(t, v.Stake, v2.Stake)
	assert.Equal(t, v.DelegatedStake, v2.DelegatedStake)
	assert.Equal(t, v.ComputeReputation, v2.ComputeReputation)

	// Update stake
	err = vr.UpdateStake(addr, 200)
	assert.Nil(t, err)
	v2, _ = vr.GetValidator(addr)
	assert.Equal(t, uint64(200), v2.Stake)

	// Delegate stake
	err = vr.DelegateStake(addr, 25)
	assert.Nil(t, err)
	v2, _ = vr.GetValidator(addr)
	assert.Equal(t, uint64(75), v2.DelegatedStake)

	// Update reputation
	err = vr.UpdateReputation(addr, 99)
	assert.Nil(t, err)
	v2, _ = vr.GetValidator(addr)
	assert.Equal(t, uint64(99), v2.ComputeReputation)

	// List validators
	vals, err := vr.ListValidators()
	assert.Nil(t, err)
	assert.Equal(t, 1, len(vals))
	assert.Equal(t, addr, vals[0].Address)
}
