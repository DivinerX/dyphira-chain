package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"encoding/json"
	"fmt"
	"io"
	"log"
	mathrand "math/rand"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/stretchr/testify/require"
)

// Intercepts log output for a specific node
type testLogger struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (tl *testLogger) Write(p []byte) (n int, err error) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	return tl.buf.Write(p)
}

func (tl *testLogger) String() string {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	return tl.buf.String()
}

// setupTestNodes creates a given number of nodes for integration testing.
func setupTestNodes(t *testing.T, numNodes int) []*AppNode {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	var nodes []*AppNode
	var loggers []*testLogger
	writers := []io.Writer{os.Stderr}

	// --- Shared validator store for all nodes ---
	sharedValidatorsDBPath, err := os.MkdirTemp("", "dyphira_shared_validators")
	require.NoError(t, err)
	validatorsDBPath := filepath.Join(sharedValidatorsDBPath, "validators.db")
	validatorStore, err := NewBoltStore(validatorsDBPath, "validators")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(sharedValidatorsDBPath) })

	// --- 1. Create all nodes (register as validators, but do not start yet) ---
	for i := 0; i < numNodes; i++ {
		port := 9000 + i
		tempDir, err := os.MkdirTemp("", fmt.Sprintf("dyphira_test_%d", port))
		require.NoError(t, err)
		chainDBPath := filepath.Join(tempDir, "chain.db")
		t.Cleanup(func() { os.RemoveAll(tempDir) })

		seed := int64(i + 1)
		r := mathrand.New(mathrand.NewSource(seed))
		p2pPrivKey, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 256, r)
		require.NoError(t, err)

		privKey, err := ecdsa.GenerateKey(elliptic.P256(), r)
		require.NoError(t, err)

		pubKeyBytes, err := p2pPrivKey.GetPublic().Raw()
		require.NoError(t, err)
		var addr Address
		copy(addr[:], pubKeyBytes[:20])

		chainStore, err := NewBoltStore(chainDBPath, "chain")
		require.NoError(t, err)

		node, err := NewAppNodeWithStores(ctx, port, p2pPrivKey, privKey, chainStore, validatorStore)
		require.NoError(t, err)

		logger := &testLogger{}
		loggers = append(loggers, logger)
		writers = append(writers, logger)

		nodes = append(nodes, node)
	}

	log.SetOutput(io.MultiWriter(writers...))
	t.Cleanup(func() {
		log.SetOutput(os.Stderr)
		if t.Failed() {
			for i, logger := range loggers {
				t.Logf("--- Logs for Node %d ---", i)
				t.Log(logger.String())
			}
		}
	})

	// --- 2. Start and connect all nodes ---
	for i, node := range nodes {
		require.NoError(t, node.Start())
		for j := i + 1; j < len(nodes); j++ {
			peerNode := nodes[j]
			peerAddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", 9000+j, peerNode.p2p.host.ID())
			err := node.p2p.Connect(ctx, peerAddr)
			require.NoError(t, err, "failed to connect node %d to node %d", i, j)
		}
	}

	time.Sleep(2 * time.Second) // Give nodes a moment to establish connections and elect a committee

	// Synchronize committee and proposer selector across all nodes
	committee, _ := nodes[0].vr.ListValidators()
	if len(committee) > numNodes {
		committee = committee[:numNodes]
	}
	for _, node := range nodes {
		node.committee = committee
		node.proposerSelector = NewProposerSelectorWithRotation(committee, node.bc.Height(), EpochLength)
	}

	// Set test hook for committee sync on validator replacement
	TestSyncCommittee = func(newCommittee []*Validator) {
		for _, node := range nodes {
			node.committee = newCommittee
			node.proposerSelector = NewProposerSelectorWithRotation(newCommittee, node.bc.Height(), EpochLength)
		}
	}

	return nodes
}

func TestDPoS_BlockApprovalAndFinalization(t *testing.T) {
	nodes := setupTestNodes(t, 4)

	// Wait for the first block to be finalized
	timeout := 30 * time.Second
	require.Eventually(t, func() bool {
		for _, n := range nodes {
			if n.bc.Height() < 1 {
				return false
			}
		}
		return true
	}, timeout, 1*time.Second, "all nodes should finalize the block within the timeout")

	// Check that all nodes have the same block at height 1
	firstNodeBlock, err := nodes[0].bc.GetBlockByHeight(1)
	require.NoError(t, err)
	for i := 1; i < len(nodes); i++ {
		otherNodeBlock, err := nodes[i].bc.GetBlockByHeight(1)
		require.NoError(t, err)
		require.Equal(t, firstNodeBlock.Header.Hash, otherNodeBlock.Header.Hash, "all nodes must have the same block at height 1")
	}
}

func TestDPoS_TransactionInclusion(t *testing.T) {
	nodes := setupTestNodes(t, 5)

	// Give each node some funds in the genesis state
	for _, n := range nodes {
		addr := pubKeyToAddress(&n.privKey.PublicKey)
		acc := &Account{Address: addr, Balance: 1000, Nonce: 0}
		require.NoError(t, n.state.PutAccount(acc))
	}

	// Wait for the first block to be created to ensure the network is running
	require.Eventually(t, func() bool {
		for _, n := range nodes {
			if n.bc.Height() < 1 {
				return false
			}
		}
		return true
	}, 15*time.Second, 1*time.Second, "all nodes should create the first block")

	// Node 0 creates and broadcasts a transaction
	tx := &Transaction{
		To:    pubKeyToAddress(&nodes[1].privKey.PublicKey),
		Value: 10,
		Nonce: 1, // First tx from this account
	}
	require.NoError(t, nodes[0].BroadcastTransaction(tx))

	// Wait for the next block (height 2) which should include the transaction
	require.Eventually(t, func() bool {
		for _, n := range nodes {
			if n.bc.Height() < 2 {
				return false
			}
		}
		return true
	}, 15*time.Second, 1*time.Second, "all nodes should create the second block")

	// Verify the transaction is in the block on all nodes
	for _, n := range nodes {
		block, err := n.bc.GetBlockByHeight(2)
		require.NoError(t, err)
		require.Len(t, block.Transactions, 1, "block should contain one transaction")
		require.Equal(t, tx.Hash, block.Transactions[0].Hash, "transaction hash must match")
	}
}

func TestDPoS_EmptyBlockOnTimeout(t *testing.T) {
	nodes := setupTestNodes(t, 3)

	// Wait for the first block to be created to ensure the network is running
	require.Eventually(t, func() bool {
		for _, n := range nodes {
			if n.bc.Height() < 1 {
				return false
			}
		}
		return true
	}, 15*time.Second, 1*time.Second, "all nodes should create the first block")

	// Simulate a block proposal that will not be approved by removing all committee members except the proposer
	proposer := nodes[0].committee[0]
	for _, n := range nodes {
		n.committee = []*Validator{proposer}
	}

	// Node 0 proposes a block, but no one else can approve it
	txs := []*Transaction{} // Empty block
	block, err := nodes[0].bc.CreateBlock(txs, proposer, nodes[0].privKey)
	require.NoError(t, err)
	blockBytes, err := json.Marshal(block)
	require.NoError(t, err)
	require.NoError(t, nodes[0].p2p.Publish(nodes[0].ctx, BlockTopic, blockBytes))

	// Wait for the timeout and empty block production
	time.Sleep(1 * time.Second)

	// Check that an empty block was produced at the next height
	emptyBlockFound := false
	for _, n := range nodes {
		b, err := n.bc.GetBlockByHeight(block.Header.BlockNumber + 1)
		if err == nil && b != nil && len(b.Transactions) == 0 {
			emptyBlockFound = true
			break
		}
	}
	require.True(t, emptyBlockFound, "An empty block should be produced on timeout")
}

func TestReplaceInactiveValidator(t *testing.T) {
	// 1. Setup
	vr := NewValidatorRegistry(NewMemoryStore(), "validators")
	committee := make([]*Validator, 4)
	for i := 0; i < 4; i++ {
		v := &Validator{Address: Address{byte(i)}, Stake: 100, Participating: true}
		committee[i] = v
		_ = vr.RegisterValidator(v)
	}
	// Add a 5th, non-committee validator to the registry to act as a replacement
	_ = vr.RegisterValidator(&Validator{Address: Address{byte(5)}, Stake: 100, Participating: true})

	inactiveValidator := committee[0]

	// 2. Execute
	newCommittee, err := ReplaceInactiveValidator(committee, inactiveValidator.Address, vr)

	// 3. Assert
	require.Nil(t, err)
	require.NotNil(t, newCommittee)
	require.Equal(t, 4, len(newCommittee), "New committee should still have 4 members")

	// Check that the inactive validator is gone
	foundInactive := false
	for _, v := range newCommittee {
		if v.Address == inactiveValidator.Address {
			foundInactive = true
			break
		}
	}
	require.False(t, foundInactive, "Inactive validator should not be in the new committee")

	// Check that a new validator was added
	foundNew := false
	for _, v := range newCommittee {
		if v.Address == (Address{byte(5)}) {
			foundNew = true
			break
		}
	}
	require.True(t, foundNew, "A new validator should have been added from the registry")
}
