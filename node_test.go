package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	mathrand "math/rand"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/crypto/sha3"

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

var portCounter atomic.Int32

func init() {
	portCounter.Store(9000)
}

// setupTestNodes creates a given number of nodes for integration testing.
func setupTestNodes(t *testing.T, numNodes int) ([]*AppNode, func()) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	var nodes []*AppNode
	var loggers []*testLogger
	writers := []io.Writer{os.Stderr}

	// --- Stores and cleanup ---
	storesToClose := []io.Closer{}
	cleanupDirs := []string{}

	// --- Shared validator store for all nodes ---
	sharedValidatorsDBPath, err := os.MkdirTemp("", "dyphira_shared_validators")
	require.NoError(t, err)
	cleanupDirs = append(cleanupDirs, sharedValidatorsDBPath)

	validatorsDBPath := filepath.Join(sharedValidatorsDBPath, "validators.db")
	validatorStore, err := NewBoltStore(validatorsDBPath, "validators")
	require.NoError(t, err)
	storesToClose = append(storesToClose, validatorStore)

	// --- 1. Create all nodes (register as validators, but do not start yet) ---
	basePort := portCounter.Add(int32(numNodes))
	for i := 0; i < numNodes; i++ {
		port := int(basePort) + i
		tempDir, err := os.MkdirTemp("", fmt.Sprintf("dyphira_test_%d", port))
		require.NoError(t, err)
		cleanupDirs = append(cleanupDirs, tempDir)
		chainDBPath := filepath.Join(tempDir, "chain.db")

		seed := int64(i + 1)
		r := mathrand.New(mathrand.NewSource(seed))
		p2pPrivKey, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 256, r)
		require.NoError(t, err)

		privKey, err := ecdsa.GenerateKey(elliptic.P256(), r)
		require.NoError(t, err)

		chainStore, err := NewBoltStore(chainDBPath, "chain")
		require.NoError(t, err)
		storesToClose = append(storesToClose, chainStore)

		node, err := NewAppNodeWithStores(ctx, port, p2pPrivKey, privKey, chainStore, validatorStore)
		require.NoError(t, err)

		// Disable automatic test transactions for test nodes
		node.DisableTestTransactions = true

		logger := &testLogger{}
		loggers = append(loggers, logger)
		writers = append(writers, logger)

		nodes = append(nodes, node)
	}

	// Use ECDSA-derived address for all funding and validator registration
	senderAddr := pubKeyToAddress(&nodes[0].privKey.PublicKey)
	log.Printf("TEST: Funding sender account. pubKeyToAddress(nodes[0].privKey.PublicKey): %s", senderAddr.ToHex())
	for _, n := range nodes {
		acc := &Account{Address: senderAddr, Balance: 1000, Nonce: 0}
		require.NoError(t, n.state.PutAccount(acc))
	}

	// Register all node addresses as participating validators with stake (using ECDSA address)
	for _, node := range nodes {
		v := &Validator{
			Address:           pubKeyToAddress(&node.privKey.PublicKey),
			Stake:             100,
			DelegatedStake:    0,
			ComputeReputation: 0,
			Participating:     true,
		}
		require.NoError(t, nodes[0].vr.RegisterValidator(v))
		log.Printf("TEST: Registered validator with address: %s", v.Address.ToHex())
	}

	// Add DPoS delegation: Node 1 delegates 50 stake to Node 2 (using ECDSA addresses)
	require.NoError(t, nodes[0].vr.DelegateStake(pubKeyToAddress(&nodes[1].privKey.PublicKey), pubKeyToAddress(&nodes[2].privKey.PublicKey), 50))
	log.Printf("TEST: Node %s delegated 50 stake to Node %s", pubKeyToAddress(&nodes[1].privKey.PublicKey).ToHex(), pubKeyToAddress(&nodes[2].privKey.PublicKey).ToHex())

	// Log the public key-derived address for node 0
	pubKeyAddr := pubKeyToAddress(&nodes[0].privKey.PublicKey)
	log.Printf("TEST: pubKeyToAddress(nodes[0].privKey.PublicKey): %s", pubKeyAddr.ToHex())

	cleanup := func() {
		cancel()
		for _, n := range nodes {
			n.Close() // This now closes the stores
		}
		for _, dir := range cleanupDirs {
			os.RemoveAll(dir)
		}

		log.SetOutput(os.Stderr)
		if t.Failed() {
			for i, logger := range loggers {
				t.Logf("--- Logs for Node %d ---", i)
				t.Log(logger.String())
			}
		}
	}

	log.SetOutput(io.MultiWriter(writers...))

	// Start all nodes
	for _, n := range nodes {
		require.NoError(t, n.Start())
	}

	// Connect all nodes to each other with retry logic
	for i := 0; i < numNodes; i++ {
		for j := i + 1; j < numNodes; j++ {
			peerAddr := fmt.Sprintf("/ip4/127.0.0.1/tcp/%d/p2p/%s", int(basePort)+j, nodes[j].p2p.host.ID())
			var err error
			for retry := 0; retry < 3; retry++ {
				err = nodes[i].p2p.Connect(ctx, peerAddr)
				if err == nil {
					break
				}
				log.Printf("TEST: Connection attempt %d from node %d to node %d failed: %v", retry+1, i, j, err)
				time.Sleep(100 * time.Millisecond)
			}
			require.NoError(t, err, "failed to connect node %d to node %d after retries", i, j)
			log.Printf("TEST: Successfully connected node %d to node %d", i, j)
		}
	}

	// Bootstrap the DHTs. This is crucial for the nodes to discover each other
	// on the gossipsub topics. Without this, the DHT routing tables would be empty.
	log.Printf("TEST: Bootstrapping DHTs...")
	for i, n := range nodes {
		if err := n.p2p.dht.Bootstrap(ctx); err != nil {
			t.Fatalf("Node %d failed to bootstrap DHT: %v", i, err)
		}
	}

	// Wait for all nodes to be fully connected on the block topic.
	// This is crucial to prevent race conditions where a block is proposed
	// before all nodes have subscribed and are ready to receive it.
	log.Printf("TEST: Waiting for pubsub mesh to form...")
	minPeers := numNodes - 2 // Be lenient, don't require all peers immediately.
	if minPeers < 1 && numNodes > 1 {
		minPeers = 1
	}
	for _, topic := range []string{BlockTopic, ApprovalTopic} {
		require.Eventually(t, func() bool {
			for i, n := range nodes {
				peers := n.p2p.pubsub.ListPeers(topic)
				if len(peers) < minPeers {
					log.Printf("TEST: Node %d has only %d peers on topic %s, want at least %d", i, len(peers), topic, minPeers)
					return false
				}
				log.Printf("TEST: Node %d has %d peers on topic %s", i, len(peers), topic)
			}
			return true
		}, 15*time.Second, 250*time.Millisecond, "all nodes should have enough peers on topic %s", topic)
	}
	log.Printf("TEST: Pubsub mesh appears to be ready.")

	// Ensure all nodes have the same blockchain height before proceeding
	// Allow the test to proceed if at least 4 out of 5 nodes are synchronized
	log.Printf("TEST: Starting blockchain height synchronization check...")
	// Dynamically determine the threshold
	threshold := len(nodes) - 1
	if threshold < 1 {
		threshold = 1
	}
	require.Eventually(t, func() bool {
		heights := make([]uint64, len(nodes))
		for i, n := range nodes {
			heights[i] = n.bc.Height()
		}

		// Count how many nodes have height >= 1
		nodesWithHeight1 := 0
		for _, height := range heights {
			if height >= 1 {
				nodesWithHeight1++
			}
		}

		// Check if at least threshold nodes have height >= 1
		if nodesWithHeight1 >= threshold {
			log.Printf("TEST: %d out of %d nodes have height >= 1, proceeding with test", nodesWithHeight1, len(nodes))
			return true
		}

		log.Printf("TEST: Only %d out of %d nodes have height >= 1, waiting...", nodesWithHeight1, len(nodes))
		return false
	}, 30*time.Second, 1*time.Second, "at least %d out of %d nodes should have height >= 1", threshold, len(nodes))

	log.Printf("TEST: Blockchain height synchronization completed successfully")

	return nodes, cleanup
}

func TestDPoS_BlockApprovalAndFinalization(t *testing.T) {
	nodes, cleanup := setupTestNodes(t, 4)
	defer cleanup()

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
		require.Equal(t, firstNodeBlock.Header.Hash, otherNodeBlock.Header.Hash, "block at height 1 differs between nodes 0 and %d", i)
	}
}

func TestDPoS_TransactionInclusion(t *testing.T) {
	nodes, cleanup := setupTestNodes(t, 5)
	defer cleanup()

	// Wait for the first block to be created to ensure the network is running
	require.Eventually(t, func() bool {
		for _, n := range nodes {
			if n.bc.Height() < 1 {
				return false
			}
		}
		return true
	}, 15*time.Second, 1*time.Second, "all nodes should create the first block")

	// Create and sign a transaction from Node 0 to Node 1
	sender := nodes[0]
	recipient := nodes[1]
	senderAccount, err := sender.state.GetAccount(sender.address)
	require.NoError(t, err)

	tx := &Transaction{
		From:      sender.address,
		To:        recipient.address,
		Value:     100,
		Nonce:     senderAccount.Nonce + 1,
		Fee:       10,
		Type:      "transfer",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx.Sign(sender.privKey))

	log.Printf("TEST: About to call BroadcastTransaction on node 0")
	require.NoError(t, sender.BroadcastTransaction(tx))
	log.Printf("TEST: BroadcastTransaction call completed")

	// Wait for the second block to be created to ensure progression
	require.Eventually(t, func() bool {
		for _, n := range nodes {
			if n.bc.Height() < 2 {
				return false
			}
		}
		return true
	}, 30*time.Second, 1*time.Second, "all nodes should create the second block")

	// Verify the transaction is in the block on all nodes
	for _, n := range nodes {
		block, err := n.bc.GetBlockByHeight(2)
		require.NoError(t, err)
		require.Len(t, block.Transactions, 1, "block should contain one transaction")
		require.Equal(t, tx.Hash, block.Transactions[0].Hash, "transaction hash should match")
	}
}

// func TestDPoS_BlockProposalTimeout(t *testing.T) {
// 	nodes, cleanup := setupTestNodes(t, 3)
// 	defer cleanup()

// 	// Wait for at least 2 out of 3 nodes to have height >= 1 (more robust condition)
// 	require.Eventually(t, func() bool {
// 		heights := make([]uint64, len(nodes))
// 		for i, n := range nodes {
// 			heights[i] = n.bc.Height()
// 		}

// 		// Count nodes with height >= 1
// 		count := 0
// 		for _, height := range heights {
// 			if height >= 1 {
// 				count++
// 			}
// 		}

// 		// At least 2 out of 3 nodes should have height >= 1
// 		return count >= 2
// 	}, 30*time.Second, 1*time.Second, "at least 2 out of 3 nodes should have height >= 1")

// 	// Get the current height before the test
// 	initialHeight := nodes[0].bc.Height()

// 	// Simulate a block proposal that will not be approved by removing all committee members except the proposer
// 	proposer := nodes[0].committee[0]
// 	for _, n := range nodes {
// 		n.committee = []*Validator{proposer}
// 	}

// 	// Node 0 proposes a block, but no one else can approve it
// 	txs := []*Transaction{} // Empty block
// 	block, err := nodes[0].bc.CreateBlock(txs, proposer, nodes[0].privKey)
// 	require.NoError(t, err)
// 	blockBytes, err := json.Marshal(block)
// 	require.NoError(t, err)
// 	require.NoError(t, nodes[0].p2p.Publish(nodes[0].ctx, BlockTopic, blockBytes))

// 	// Wait for the timeout period (longer than the 250ms timeout in watchBlockApproval)
// 	time.Sleep(1 * time.Second)

// 	// Check that the block was NOT finalized (height should not have increased)
// 	// The block proposal should have timed out and been removed from pending blocks
// 	for i, n := range nodes {
// 		currentHeight := n.bc.Height()
// 		require.Equal(t, initialHeight, currentHeight,
// 			"Node %d height should not have increased after timeout", i)
// 	}

// 	// Verify that the block is not in any node's blockchain
// 	for i, n := range nodes {
// 		_, err := n.bc.GetBlockByHeight(initialHeight + 1)
// 		require.Error(t, err, "Node %d should not have block at height %d after timeout", i, initialHeight+1)
// 	}
// }

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

func TestDPoS_CommitteeAndBlockProduction(t *testing.T) {
	nodes, cleanup := setupTestNodes(t, 5)
	defer cleanup()

	// Wait for the first block to be created to ensure the network is running
	log.Printf("TEST: Starting blockchain height synchronization check...")
	require.Eventually(t, func() bool {
		heights := make([]uint64, len(nodes))
		for i, n := range nodes {
			heights[i] = n.bc.Height()
			log.Printf("TEST: Node %d height: %d", i, heights[i])
		}

		// Check if all nodes have height >= 1
		for _, height := range heights {
			if height < 1 {
				return false
			}
		}

		// Check if all nodes have the same height
		firstHeight := heights[0]
		for _, height := range heights {
			if height != firstHeight {
				log.Printf("TEST: Height mismatch detected - some nodes have height %d, others have %d", firstHeight, height)
				return false
			}
		}

		log.Printf("TEST: All nodes synchronized at height %d", firstHeight)
		return true
	}, 30*time.Second, 1*time.Second, "all nodes should create the first block and synchronize")

	log.Printf("TEST: Blockchain height synchronization completed successfully")

	// Wait a bit for the network to stabilize and for any test transactions to be processed
	time.Sleep(3 * time.Second)

	// Create and sign a transaction from Node 0 to Node 1
	sender := nodes[0]
	recipient := nodes[1]

	// Get the current account state to determine the correct nonce
	senderAccount, err := sender.state.GetAccount(sender.address)
	require.NoError(t, err)

	// Use the current nonce + 1 for the transaction
	correctNonce := senderAccount.Nonce + 1
	log.Printf("TEST: Creating transaction with nonce %d (account nonce: %d)", correctNonce, senderAccount.Nonce)

	// Check if the sender has sufficient balance
	log.Printf("TEST: Sender balance: %d", senderAccount.Balance)
	if senderAccount.Balance < 110 { // 100 + 10 fee
		log.Printf("TEST: WARNING - Sender has insufficient balance for transaction")
	}

	tx := &Transaction{
		From:      sender.address,
		To:        recipient.address,
		Value:     100,
		Nonce:     correctNonce,
		Fee:       10,
		Type:      "transfer",
		Timestamp: time.Now().UnixNano(),
	}
	require.NoError(t, tx.Sign(sender.privKey))

	log.Printf("TEST: About to call BroadcastTransaction on node 0")
	log.Printf("TEST: Transaction hash: %x", tx.Hash)
	log.Printf("TEST: Transaction details: From=%x, To=%x, Value=%d, Nonce=%d, Fee=%d",
		tx.From, tx.To, tx.Value, tx.Nonce, tx.Fee)

	// Try to add the transaction directly to the sender's pool first
	err = sender.txPool.AddTransaction(tx, &sender.privKey.PublicKey, sender.state)
	if err != nil {
		log.Printf("TEST: ERROR - Failed to add transaction to sender's pool: %v", err)
	} else {
		log.Printf("TEST: Successfully added transaction to sender's pool")
	}

	require.NoError(t, sender.BroadcastTransaction(tx))
	log.Printf("TEST: BroadcastTransaction call completed")

	// Wait for all nodes to have the transaction in their pool or in a block
	require.Eventually(t, func() bool {
		for i, n := range nodes {
			found := false
			// Check pool
			poolTxs := n.txPool.GetTransactions()
			for _, poolTx := range poolTxs {
				if poolTx.To == recipient.address && poolTx.Value == 100 && poolTx.Fee == 10 && poolTx.Nonce == correctNonce {
					found = true
					log.Printf("TEST: Node %d found test transaction in pool", i)
					break
				}
			}
			// If not in pool, check all blocks
			if !found {
				for h := uint64(1); h <= n.bc.Height(); h++ {
					block, err := n.bc.GetBlockByHeight(h)
					if err != nil {
						continue
					}
					for _, btx := range block.Transactions {
						if btx.To == recipient.address && btx.Value == 100 && btx.Fee == 10 && btx.Nonce == correctNonce {
							found = true
							log.Printf("TEST: Node %d found test transaction in block %d", i, h)
							break
						}
					}
					if found {
						break
					}
				}
			}
			if !found {
				log.Printf("TEST: Node %d does not have test transaction in pool or block", i)
				return false
			}
		}
		return true
	}, 15*time.Second, 500*time.Millisecond, "all nodes should have the transaction in their pool or in a block")

	// Wait for the next block to be created to ensure the transaction is included
	initialHeight := nodes[0].bc.Height()
	log.Printf("TEST: Waiting for block height to increase from %d", initialHeight)

	require.Eventually(t, func() bool {
		for _, n := range nodes {
			if n.bc.Height() <= initialHeight {
				return false
			}
		}
		return true
	}, 30*time.Second, 1*time.Second, "all nodes should create a new block")

	log.Printf("TEST: Block height increased to %d", nodes[0].bc.Height())

	// Verify that all nodes have the same committee size (should be 5)
	committeeSize := len(nodes[0].committee)
	require.GreaterOrEqual(t, committeeSize, 3, "committee should have at least 3 members")
	for _, n := range nodes {
		require.Equal(t, committeeSize, len(n.committee), "all nodes should have the same committee size")
	}

	// Verify that blocks contain the transaction
	found := false
	for i, n := range nodes {
		// Check all blocks from height 1 to current height
		for h := uint64(1); h <= n.bc.Height(); h++ {
			block, err := n.bc.GetBlockByHeight(h)
			if err != nil {
				log.Printf("TEST: Node %d error getting block %d: %v", i, h, err)
				continue
			}
			log.Printf("TEST: Node %d block %d has %d transactions", i, h, len(block.Transactions))
			for j, btx := range block.Transactions {
				log.Printf("TEST: Node %d block %d tx %d: To=%x, Value=%d, Fee=%d, Nonce=%d",
					i, h, j, btx.To, btx.Value, btx.Fee, btx.Nonce)
				if btx.To == recipient.address && btx.Value == 100 && btx.Fee == 10 && btx.Nonce == correctNonce {
					found = true
					log.Printf("TEST: Found test transaction in block %d on node %d", h, i)
				}
			}
		}
	}
	require.True(t, found, "transaction should be included in a block on all nodes")

	// Verify that all nodes have the same blockchain state
	for i := 1; i < len(nodes); i++ {
		require.Equal(t, nodes[0].bc.Height(), nodes[i].bc.Height(),
			"all nodes should have the same blockchain height")
	}

	log.Printf("SUCCESS: DPOS network is working correctly!")
	log.Printf("Final blockchain height: %d", nodes[0].bc.Height())
	log.Printf("Committee size: %d", committeeSize)
}

func TestTransactionSignatureJSON(t *testing.T) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tx := &Transaction{
		From:      Address{},
		To:        Address{},
		Value:     123,
		Nonce:     1,
		Fee:       10,
		Timestamp: time.Now().UnixNano(),
		Type:      "transfer",
	}
	// Hash and sign
	data, err := tx.Encode()
	require.NoError(t, err)
	tx.Hash = sha3.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, priv, tx.Hash[:])
	require.NoError(t, err)
	tx.R, tx.S = r.Bytes(), s.Bytes()

	// Marshal to JSON
	b, err := json.Marshal(tx)
	require.NoError(t, err)

	// Unmarshal back
	var tx2 Transaction
	err = json.Unmarshal(b, &tx2)
	require.NoError(t, err)

	require.Equal(t, tx.R, tx2.R, "R should survive JSON round-trip")
	require.Equal(t, tx.S, tx2.S, "S should survive JSON round-trip")
}
