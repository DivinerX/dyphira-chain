package main

import (
	"encoding/json"
	"strings"
)

type ValidatorRegistry struct {
	store  Storage
	bucket string
}

func NewValidatorRegistry(store Storage, bucket string) *ValidatorRegistry {
	return &ValidatorRegistry{store: store, bucket: bucket}
}

func (vr *ValidatorRegistry) validatorKey(addr Address) []byte {
	return append([]byte("validator_"), addr[:]...)
}

func (vr *ValidatorRegistry) RegisterValidator(v *Validator) error {
	if v == nil {
		return nil
	}
	key := vr.validatorKey(v.Address)
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return vr.store.Put(key, data)
}

func (vr *ValidatorRegistry) GetValidator(addr Address) (*Validator, error) {
	key := vr.validatorKey(addr)
	data, err := vr.store.Get(key)
	if err != nil {
		// If the error indicates the key was not found, return nil for both.
		// This is a common pattern to distinguish "not found" from other errors.
		if strings.Contains(err.Error(), "not found") {
			return nil, nil
		}
		return nil, err
	}
	if data == nil {
		return nil, nil // Not found
	}
	var v Validator
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, err
	}
	return &v, nil
}

func (vr *ValidatorRegistry) UpdateStake(addr Address, stake uint64) error {
	v, err := vr.GetValidator(addr)
	if err != nil {
		return err
	}
	v.Stake = stake
	return vr.RegisterValidator(v)
}

func (vr *ValidatorRegistry) DelegateStake(delegator, validator Address, amount uint64) error {
	v, err := vr.GetValidator(validator)
	if err != nil {
		return err
	}
	if v == nil {
		return err // Or a more specific error like "validator not found"
	}
	v.DelegatedStake += amount
	return vr.RegisterValidator(v)
}

func (vr *ValidatorRegistry) UpdateReputation(addr Address, rep uint64) error {
	v, err := vr.GetValidator(addr)
	if err != nil {
		return err
	}
	v.ComputeReputation = rep
	return vr.RegisterValidator(v)
}

// GetAllValidators returns all registered validators.
func (vr *ValidatorRegistry) GetAllValidators() ([]*Validator, error) {
	var validators []*Validator
	allData, err := vr.store.List()
	if err != nil {
		return nil, err
	}

	prefix := "validator_"
	for key, data := range allData {
		if strings.HasPrefix(key, prefix) {
			var v Validator
			if err := json.Unmarshal(data, &v); err != nil {
				// Log or handle corrupted data
				continue
			}
			validators = append(validators, &v)
		}
	}
	return validators, nil
}

// ClearAllValidators removes all validators from the registry.
func (vr *ValidatorRegistry) ClearAllValidators() error {
	allData, err := vr.store.List()
	if err != nil {
		return err
	}

	prefix := "validator_"
	for key := range allData {
		if strings.HasPrefix(key, prefix) {
			if err := vr.store.Delete([]byte(key)); err != nil {
				return err
			}
		}
	}
	return nil
}
