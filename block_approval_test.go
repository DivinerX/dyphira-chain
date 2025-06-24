package main

import (
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	ecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/stretchr/testify/assert"
)

func TestBlockApproval(t *testing.T) {
	// 1. Setup
	numValidators := 10
	committee := make([]*Validator, numValidators)
	privKeys := make([]*btcec.PrivateKey, numValidators)

	for i := 0; i < numValidators; i++ {
		priv, _ := btcec.NewPrivateKey()
		addr := pubKeyToAddress(priv.PubKey())
		committee[i] = &Validator{Address: addr}
		privKeys[i] = priv
	}

	block := &Block{Header: &Header{Hash: Hash{1}}}
	ba := NewBlockApproval(block, committee)

	// 2. Test Threshold
	// Threshold should be (2 * 10 / 3) + 1 = 6 + 1 = 7
	assert.Equal(t, 7, ba.Threshold)

	// 3. Add signatures up to threshold - 1
	for i := 0; i < ba.Threshold-1; i++ {
		sig := ecdsa.Sign(privKeys[i], block.Header.Hash[:])
		ba.AddSignature(committee[i].Address, sig.Serialize())
		assert.False(t, ba.IsApproved())
	}

	// 4. Add the final signature to meet threshold
	sig := ecdsa.Sign(privKeys[ba.Threshold-1], block.Header.Hash[:])
	ba.AddSignature(committee[ba.Threshold-1].Address, sig.Serialize())

	// 5. Assert block is now approved
	assert.True(t, ba.IsApproved())
}
