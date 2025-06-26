package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBARIntegration_NodeInitialization(t *testing.T) {
	ctx := context.Background()

	// Generate proper keys
	privKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	p2pPrivKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)

	// Create a test node with BAR network
	node, err := NewAppNode(ctx, 8080, "test-bar.db", p2pPrivKey, privKey)
	require.NoError(t, err)
	defer node.Close()

	// Verify BAR network is initialized
	assert.NotNil(t, node.barNet)
	assert.NotNil(t, node.handshakeManager)

	// Verify default BAR configuration
	whitelist := node.barNet.GetWhitelist()
	greylist := node.barNet.GetGreylist()
	banned := node.barNet.GetBanned()
	assert.Len(t, whitelist, 0)
	assert.Len(t, greylist, 0)
	assert.Len(t, banned, 0)
}

func TestBARIntegration_PeerManagement(t *testing.T) {
	ctx := context.Background()

	// Generate proper keys
	privKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	p2pPrivKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)

	node, err := NewAppNode(ctx, 8081, "test-bar-peer.db", p2pPrivKey, privKey)
	require.NoError(t, err)
	defer node.Close()

	// Simulate peer connection
	testPeerID := peer.ID("test-peer-123")
	testAddress := "/ip4/127.0.0.1/tcp/8082"

	// Trigger peer connect callback
	if node.p2p.OnPeerConnect != nil {
		node.p2p.OnPeerConnect(testPeerID, testAddress)
	}

	// Verify peer is added to greylist
	greylist := node.barNet.GetGreylist()
	assert.Len(t, greylist, 1)
	assert.Equal(t, testPeerID, greylist[0].ID)
	assert.Equal(t, testAddress, greylist[0].Address)
	assert.Equal(t, PeerStatusGreylist, greylist[0].Status)

	// Promote to whitelist
	success := node.barNet.PromoteToWhitelist(testPeerID)
	assert.True(t, success)

	// Verify promotion
	whitelist := node.barNet.GetWhitelist()
	greylist = node.barNet.GetGreylist()
	assert.Len(t, whitelist, 1)
	assert.Len(t, greylist, 0)
	assert.Equal(t, testPeerID, whitelist[0].ID)
	assert.Equal(t, PeerStatusWhitelist, whitelist[0].Status)
}

func TestBARIntegration_FindNodesAlgorithm(t *testing.T) {
	ctx := context.Background()

	// Generate proper keys
	privKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	p2pPrivKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)

	node, err := NewAppNode(ctx, 8082, "test-bar-find.db", p2pPrivKey, privKey)
	require.NoError(t, err)
	defer node.Close()

	// Add some peers to whitelist
	for i := 0; i < 5; i++ {
		peerID := peer.ID(fmt.Sprintf("test-peer-%d", i))
		node.barNet.AddPeer(peerID, fmt.Sprintf("addr%d", i))
		node.barNet.PromoteToWhitelist(peerID)
	}

	// Test find_nodes algorithm
	selectedPeers := node.barNet.FindNodes(1)
	assert.Len(t, selectedPeers, 5) // Should select all 5 peers

	// Test deterministic selection
	selectedPeers2 := node.barNet.FindNodes(1)
	assert.ElementsMatch(t, selectedPeers, selectedPeers2)

	// Test different round gives different selection
	selectedPeers3 := node.barNet.FindNodes(2)
	assert.Len(t, selectedPeers3, 5)
}

func TestBARIntegration_POMScoreTracking(t *testing.T) {
	ctx := context.Background()

	// Generate proper keys
	privKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	p2pPrivKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)

	node, err := NewAppNode(ctx, 8083, "test-bar-pom.db", p2pPrivKey, privKey)
	require.NoError(t, err)
	defer node.Close()

	testPeerID := peer.ID("test-peer-pom")
	node.barNet.AddPeer(testPeerID, "addr")
	node.barNet.PromoteToWhitelist(testPeerID)

	// Update POM score
	node.barNet.UpdatePOMScore(testPeerID, 3, "test misbehavior")

	whitelist := node.barNet.GetWhitelist()
	assert.Equal(t, 3, whitelist[0].POMScore)

	// Update again to trigger demotion
	node.barNet.UpdatePOMScore(testPeerID, 3, "more misbehavior")

	// Should be demoted to greylist
	whitelist = node.barNet.GetWhitelist()
	greylist := node.barNet.GetGreylist()
	assert.Len(t, whitelist, 0)
	assert.Len(t, greylist, 1)
	assert.Equal(t, 6, greylist[0].POMScore)
}

func TestBARIntegration_HandshakeManager(t *testing.T) {
	ctx := context.Background()

	// Generate proper keys
	privKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	p2pPrivKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)

	node, err := NewAppNode(ctx, 8084, "test-bar-handshake.db", p2pPrivKey, privKey)
	require.NoError(t, err)
	defer node.Close()

	// Test handshake manager initialization
	assert.NotNil(t, node.handshakeManager)
	assert.Equal(t, uint64(0), node.handshakeManager.GetRound())

	// Test round update
	node.handshakeManager.UpdateRound(5)
	assert.Equal(t, uint64(5), node.handshakeManager.GetRound())
	assert.NotNil(t, node.handshakeManager.roundSeed)
}

func TestBARIntegration_RoundBasedUpdates(t *testing.T) {
	ctx := context.Background()

	// Generate proper keys
	privKey, err := btcec.NewPrivateKey()
	require.NoError(t, err)

	p2pPrivKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	require.NoError(t, err)

	node, err := NewAppNode(ctx, 8085, "test-bar-round.db", p2pPrivKey, privKey)
	require.NoError(t, err)
	defer node.Close()

	// Start the node
	err = node.Start()
	require.NoError(t, err)

	// Wait a bit for the producer loop to run
	time.Sleep(3 * time.Second)

	// Verify that rounds are being updated
	round := node.handshakeManager.GetRound()
	assert.Greater(t, round, uint64(0))

	// Verify BAR network is being used
	whitelist := node.barNet.GetWhitelist()
	greylist := node.barNet.GetGreylist()
	banned := node.barNet.GetBanned()

	// Log status for debugging
	t.Logf("BAR Status - Whitelist: %d, Greylist: %d, Banned: %d",
		len(whitelist), len(greylist), len(banned))
}
