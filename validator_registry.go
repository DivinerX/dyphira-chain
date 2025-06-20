package main

import (
	"encoding/json"

	"go.etcd.io/bbolt"
)

type ValidatorRegistry struct {
	store  IBlockStore
	bucket string
}

func NewValidatorRegistry(store IBlockStore, bucket string) *ValidatorRegistry {
	return &ValidatorRegistry{store: store, bucket: bucket}
}

func (vr *ValidatorRegistry) validatorKey(addr Address) []byte {
	return append([]byte("validator_"), addr[:]...)
}

func (vr *ValidatorRegistry) RegisterValidator(v *Validator) error {
	if v == nil {
		return nil
	}
	if !v.Participating {
		v.Participating = false
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
		return nil, err
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

func (vr *ValidatorRegistry) DelegateStake(addr Address, amount uint64) error {
	v, err := vr.GetValidator(addr)
	if err != nil {
		return err
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

// ListValidators returns all registered validators.
func (vr *ValidatorRegistry) ListValidators() ([]*Validator, error) {
	var validators []*Validator
	// For BoltStore, we need to iterate over all keys with prefix "validator_"
	if bs, ok := vr.store.(*BoltStore); ok {
		err := bs.db.View(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte("validators"))
			return b.ForEach(func(k, v []byte) error {
				if len(k) >= 10 && string(k[:10]) == "validator_" {
					var val Validator
					if err := json.Unmarshal(v, &val); err == nil {
						validators = append(validators, &val)
					}
				}
				return nil
			})
		})
		if err != nil {
			return nil, err
		}
	} else {
		// MemoryStore fallback
		for k, v := range vr.store.(*MemoryStore).data {
			if len(k) >= 10 && k[:10] == "validator_" {
				var val Validator
				if err := json.Unmarshal(v, &val); err == nil {
					validators = append(validators, &val)
				}
			}
		}
	}
	return validators, nil
}
