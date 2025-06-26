package main

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOptimisticPushManager_NewOptimisticPushManager(t *testing.T) {
	node := &AppNode{
		ctx: context.Background(),
	}

	opm := NewOptimisticPushManager(node)
	require.NotNil(t, opm)
	assert.Equal(t, node, opm.node)
}

func TestOptimisticPushMessage_Structure(t *testing.T) {
	// Create proper peer IDs for testing
	peer1 := peer.ID("test-peer-1")
	peer2 := peer.ID("test-peer-2")

	msg := &OptimisticPushMessage{
		Type:          "push_request",
		From:          peer1,
		To:            peer2,
		RequestID:     "req_123",
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "test-v1.0",
		BlockHeight:   100,
		IsLightNode:   false,
	}

	// Test that all fields are properly set
	assert.Equal(t, "push_request", msg.Type)
	assert.Equal(t, peer1, msg.From)
	assert.Equal(t, peer2, msg.To)
	assert.Equal(t, "req_123", msg.RequestID)
	assert.Equal(t, "test-v1.0", msg.ClientVersion)
	assert.Equal(t, uint64(100), msg.BlockHeight)
	assert.False(t, msg.IsLightNode)
}

func TestOptimisticPushManager_RequestOptimisticPush(t *testing.T) {
	// Create a node with BAR network but no P2P component for testing
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	opm := NewOptimisticPushManager(node)

	// Test with no whitelisted peers - will fail due to nil P2P component
	err := opm.RequestOptimisticPush(false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "P2P component not initialized")

	// Add a peer to whitelist
	testPeerID := peer.ID("test-peer-123")
	node.barNet.AddPeer(testPeerID, "addr")
	node.barNet.PromoteToWhitelist(testPeerID)

	// Test with whitelisted peer - this will fail due to nil P2P component
	err = opm.RequestOptimisticPush(false)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "P2P component not initialized")
}

func TestOptimisticPushManager_TriggerOptimisticPushForNewPeer(t *testing.T) {
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	opm := NewOptimisticPushManager(node)

	// Test triggering optimistic push for a new peer
	testPeerID := peer.ID("test-peer-123")
	opm.TriggerOptimisticPushForNewPeer(testPeerID)

	// The function should complete without error
	// In a real scenario, this would send a message to the peer
}

func TestOptimisticPushManager_MessageTypes(t *testing.T) {
	// Create proper peer IDs for testing
	peer1 := peer.ID("peer1")
	peer2 := peer.ID("peer2")

	// Test different message types
	requestMsg := &OptimisticPushMessage{
		Type:          "push_request",
		From:          peer1,
		To:            peer2,
		RequestID:     "req_1",
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "v1.0",
		IsLightNode:   false,
	}

	responseMsg := &OptimisticPushMessage{
		Type:          "push_response",
		From:          peer2,
		To:            peer1,
		RequestID:     "req_1",
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "v1.0",
		BlockHeight:   100,
		IsLightNode:   false,
	}

	blockDataMsg := &OptimisticPushMessage{
		Type:          "block_data",
		From:          peer2,
		To:            peer1,
		RequestID:     "req_1",
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "v1.0",
		BlockHeight:   100,
		IsLightNode:   false,
	}

	// Verify message types
	assert.Equal(t, "push_request", requestMsg.Type)
	assert.Equal(t, "push_response", responseMsg.Type)
	assert.Equal(t, "block_data", blockDataMsg.Type)
}

func TestOptimisticPushManager_LightNodeSupport(t *testing.T) {
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	opm := NewOptimisticPushManager(node)

	// Test light node request
	err := opm.RequestOptimisticPush(true)
	assert.Error(t, err) // Should fail with no whitelisted peers

	// Add a peer to whitelist
	testPeerID := peer.ID("test-peer-123")
	node.barNet.AddPeer(testPeerID, "addr")
	node.barNet.PromoteToWhitelist(testPeerID)

	// Test light node request with whitelisted peer - will fail due to nil P2P
	err = opm.RequestOptimisticPush(true)
	assert.Error(t, err) // Will fail due to nil P2P component
}

func TestOptimisticPushManager_Integration(t *testing.T) {
	// Create a node with blockchain
	ctx := context.Background()

	// Create a simple blockchain for testing
	chainStore, err := NewBoltStore("test_optimistic_push.db", "blocks")
	require.NoError(t, err)
	defer chainStore.Close()

	bc, err := NewBlockchain(chainStore)
	require.NoError(t, err)

	node := &AppNode{
		ctx:    ctx,
		bc:     bc,
		barNet: NewBARNetwork(nil),
	}

	opm := NewOptimisticPushManager(node)

	// Test that the manager can access blockchain methods
	height := opm.node.bc.Height()
	assert.Equal(t, uint64(0), height) // Should start at height 0

	// Test block data retrieval (should return nil for non-existent block)
	blockData := opm.getBlockData(1)
	assert.Nil(t, blockData) // No block at height 1

	// Test block headers retrieval - should return genesis block header
	headers := opm.getBlockHeaders(0)
	assert.Len(t, headers, 1) // Genesis block header
}
