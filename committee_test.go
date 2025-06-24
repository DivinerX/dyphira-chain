package main

import (
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to register validators and optionally make them participate via transaction
func makeTestValidatorsWithParticipation(vr *ValidatorRegistry, state *State, txPool *TransactionPool, n int, participateIndices ...int) ([]Address, []*btcec.PrivateKey) {
	addrs := make([]Address, n)
	privs := make([]*btcec.PrivateKey, n)
	participateMap := map[int]bool{}
	for _, idx := range participateIndices {
		participateMap[idx] = true
	}
	for i := 0; i < n; i++ {
		priv, _ := btcec.NewPrivateKey()
		addr := pubKeyToAddress(priv.PubKey())
		v := &Validator{
			Address:           addr,
			Stake:             uint64(100 + i*10),
			DelegatedStake:    uint64(i * 5),
			ComputeReputation: uint64(i),
			Participating:     false, // Always false initially
		}
		_ = vr.RegisterValidator(v)
		addrs[i] = addr
		privs[i] = priv
		// Fund account for participation tx
		acc := &Account{Address: addr, Balance: 1000, Nonce: 0}
		_ = state.PutAccount(acc)
		if participateMap[i] {
			tx := &Transaction{
				From:      addr,
				To:        addr,
				Value:     0,
				Nonce:     1,
				Fee:       0,
				Type:      "participation",
				Timestamp: time.Now().UnixNano(),
			}
			require.NoError(nil, tx.Sign(priv))
			err := txPool.AddTransaction(tx, priv.PubKey(), state)
			if err == nil {
				block := &Block{Transactions: []*Transaction{tx}}
				bc, _ := NewBlockchain(NewMemoryStore())
				_ = bc.ApplyBlockWithRegistry(block, state, vr)
			}
		}
	}
	return addrs, privs
}

func TestCommitteeSelection(t *testing.T) {
	store := NewMemoryStore()
	vr := NewValidatorRegistry(store, "validators")
	state := NewState()
	txPool := NewTransactionPool()
	_, _ = makeTestValidatorsWithParticipation(vr, state, txPool, 10, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9)
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
	state := NewState()
	txPool := NewTransactionPool()
	_, _ = makeTestValidatorsWithParticipation(vr, state, txPool, 3, 0, 1, 2)
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
	state := NewState()
	txPool := NewTransactionPool()
	_, _ = makeTestValidatorsWithParticipation(vr, state, txPool, 3, 0) // Only first validator participates
	cs := &CommitteeSelector{Registry: vr}

	committee, err := cs.SelectCommittee(3)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(committee))
	assert.True(t, committee[0].Participating)
}
