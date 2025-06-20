package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBlockApproval(t *testing.T) {
	// 1. Setup
	numValidators := 10
	committee := make([]*Validator, numValidators)
	privKeys := make([]*ecdsa.PrivateKey, numValidators)

	for i := 0; i < numValidators; i++ {
		priv, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		addr := pubKeyToAddress(&priv.PublicKey)
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
		r, s, _ := ecdsa.Sign(rand.Reader, privKeys[i], block.Header.Hash[:])
		sig := append(r.Bytes(), s.Bytes()...)
		ba.AddSignature(committee[i].Address, sig)
		assert.False(t, ba.IsApproved())
	}

	// 4. Add the final signature to meet threshold
	r, s, _ := ecdsa.Sign(rand.Reader, privKeys[ba.Threshold-1], block.Header.Hash[:])
	sig := append(r.Bytes(), s.Bytes()...)
	ba.AddSignature(committee[ba.Threshold-1].Address, sig)

	// 5. Assert block is now approved
	assert.True(t, ba.IsApproved())
}
