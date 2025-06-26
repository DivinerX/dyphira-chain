package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBARNetwork_NewBARNetwork(t *testing.T) {
	config := &BARConfig{
		OutgoingConnections: 10,
		IncomingConnections: 5,
		SyncTimeout:         15 * time.Second,
		POMThreshold:        3,
		POMBanThreshold:     10,
		ReputationMemory:    5,
		HandshakeTimeout:    8 * time.Second,
		SeedNodes:           []string{"seed1", "seed2"},
	}

	bn := NewBARNetwork(config)
	require.NotNil(t, bn)
	assert.Equal(t, config, bn.config)
	assert.Equal(t, config.SeedNodes, bn.seedNodes)
	assert.NotNil(t, bn.prngSeed)
	assert.Len(t, bn.prngSeed, 32)
}

func TestBARNetwork_AddPeer(t *testing.T) {
	bn := NewBARNetwork(nil)

	// Create test peer ID
	peerID := peer.ID("test-peer-1")
	address := "/ip4/127.0.0.1/tcp/8080"

	// Add peer
	bn.AddPeer(peerID, address)

	// Verify peer is in greylist
	greylist := bn.GetGreylist()
	assert.Len(t, greylist, 1)
	assert.Equal(t, peerID, greylist[0].ID)
	assert.Equal(t, address, greylist[0].Address)
	assert.Equal(t, PeerStatusGreylist, greylist[0].Status)
	assert.Equal(t, 0, greylist[0].POMScore)

	// Try to add same peer again - should not duplicate
	bn.AddPeer(peerID, address)
	greylist = bn.GetGreylist()
	assert.Len(t, greylist, 1)
}

func TestBARNetwork_PromoteToWhitelist(t *testing.T) {
	bn := NewBARNetwork(&BARConfig{
		IncomingConnections: 2,
	})

	peerID1 := peer.ID("test-peer-1")
	peerID2 := peer.ID("test-peer-2")
	peerID3 := peer.ID("test-peer-3")

	// Add peers to greylist
	bn.AddPeer(peerID1, "addr1")
	bn.AddPeer(peerID2, "addr2")
	bn.AddPeer(peerID3, "addr3")

	// Promote first peer
	success := bn.PromoteToWhitelist(peerID1)
	assert.True(t, success)

	// Verify promotion
	whitelist := bn.GetWhitelist()
	greylist := bn.GetGreylist()
	assert.Len(t, whitelist, 1)
	assert.Len(t, greylist, 2)
	assert.Equal(t, peerID1, whitelist[0].ID)
	assert.Equal(t, PeerStatusWhitelist, whitelist[0].Status)

	// Promote second peer
	success = bn.PromoteToWhitelist(peerID2)
	assert.True(t, success)

	// Try to promote third peer - should fail due to limit
	success = bn.PromoteToWhitelist(peerID3)
	assert.False(t, success)

	// Verify final state
	whitelist = bn.GetWhitelist()
	greylist = bn.GetGreylist()
	assert.Len(t, whitelist, 2)
	assert.Len(t, greylist, 1)
}

func TestBARNetwork_DemoteToGreylist(t *testing.T) {
	bn := NewBARNetwork(nil)

	peerID := peer.ID("test-peer-1")
	bn.AddPeer(peerID, "addr1")
	bn.PromoteToWhitelist(peerID)

	// Demote peer
	success := bn.DemoteToGreylist(peerID)
	assert.True(t, success)

	// Verify demotion
	whitelist := bn.GetWhitelist()
	greylist := bn.GetGreylist()
	assert.Len(t, whitelist, 0)
	assert.Len(t, greylist, 1)
	assert.Equal(t, peerID, greylist[0].ID)
	assert.Equal(t, PeerStatusGreylist, greylist[0].Status)

	// Try to demote non-existent peer
	success = bn.DemoteToGreylist(peer.ID("non-existent"))
	assert.False(t, success)
}

func TestBARNetwork_UpdatePOMScore(t *testing.T) {
	bn := NewBARNetwork(&BARConfig{
		POMThreshold:        3,
		POMBanThreshold:     8,
		IncomingConnections: 5, // Ensure we have enough capacity
	})

	peerID := peer.ID("test-peer-1")
	bn.AddPeer(peerID, "addr1")
	bn.PromoteToWhitelist(peerID)

	// Update POM score
	bn.UpdatePOMScore(peerID, 2, "test reason")

	whitelist := bn.GetWhitelist()
	assert.Len(t, whitelist, 1)
	assert.Equal(t, 2, whitelist[0].POMScore)

	// Update again to trigger demotion
	bn.UpdatePOMScore(peerID, 2, "another reason")

	// Should be demoted to greylist
	whitelist = bn.GetWhitelist()
	greylist := bn.GetGreylist()
	assert.Len(t, whitelist, 0)
	assert.Len(t, greylist, 1)
	assert.Equal(t, 4, greylist[0].POMScore)

	// Update to trigger ban
	bn.UpdatePOMScore(peerID, 5, "ban reason")

	// Should be banned
	whitelist = bn.GetWhitelist()
	greylist = bn.GetGreylist()
	banned := bn.GetBanned()
	assert.Len(t, whitelist, 0)
	assert.Len(t, greylist, 0)
	assert.Len(t, banned, 1)
	assert.Equal(t, peerID, banned[0].ID)
	assert.Equal(t, PeerStatusBanned, banned[0].Status)
}

func TestBARNetwork_FindNodes(t *testing.T) {
	bn := NewBARNetwork(&BARConfig{
		IncomingConnections: 3,
	})

	// Add peers to whitelist
	for i := 0; i < 5; i++ {
		peerID := peer.ID(fmt.Sprintf("test-peer-%d", i))
		bn.AddPeer(peerID, fmt.Sprintf("addr%d", i))
		bn.PromoteToWhitelist(peerID)
	}

	// Test find_nodes algorithm
	selectedPeers := bn.FindNodes(1)
	assert.Len(t, selectedPeers, 3) // Should select 3 peers (IncomingConnections)

	// Test deterministic selection - same round should give same result
	selectedPeers2 := bn.FindNodes(1)
	assert.ElementsMatch(t, selectedPeers, selectedPeers2) // Compare as sets

	// Different round should give different result
	selectedPeers3 := bn.FindNodes(2)
	assert.Len(t, selectedPeers3, 3)
}

func TestBARNetwork_GetPeerStatus(t *testing.T) {
	bn := NewBARNetwork(nil)

	peerID := peer.ID("test-peer-1")

	// Test non-existent peer
	status, exists := bn.GetPeerStatus(peerID)
	assert.False(t, exists)
	assert.Equal(t, PeerStatusGreylist, status)

	// Add peer to greylist
	bn.AddPeer(peerID, "addr1")
	status, exists = bn.GetPeerStatus(peerID)
	assert.True(t, exists)
	assert.Equal(t, PeerStatusGreylist, status)

	// Promote to whitelist
	bn.PromoteToWhitelist(peerID)
	status, exists = bn.GetPeerStatus(peerID)
	assert.True(t, exists)
	assert.Equal(t, PeerStatusWhitelist, status)

	// Ban peer
	bn.BanPeer(peerID, "test ban")
	status, exists = bn.GetPeerStatus(peerID)
	assert.True(t, exists)
	assert.Equal(t, PeerStatusBanned, status)
}

func TestBARNetwork_CleanupReputationRecords(t *testing.T) {
	bn := NewBARNetwork(&BARConfig{
		ReputationMemory: 3,
		POMBanThreshold:  10, // Set high to avoid banning
	})

	peerID := peer.ID("test-peer-1")

	// Add peer and update POM score to create reputation record (but not ban)
	bn.AddPeer(peerID, "addr1")
	bn.UpdatePOMScore(peerID, 2, "test") // Low score to avoid banning

	// Advance rounds
	bn.FindNodes(5) // This sets round to 5

	// Add a fake old record that should be cleaned up
	bn.reputationRecords[peer.ID("old-peer")] = &ReputationRecord{
		POMScore:  5,
		LastRound: 1, // Should be cleaned up (5 - 3 = 2, so 1 < 2)
		Reason:    "old",
	}

	// Add a fake recent record that should NOT be cleaned up
	bn.reputationRecords[peer.ID("recent-peer")] = &ReputationRecord{
		POMScore:  3,
		LastRound: 3, // Should NOT be cleaned up (3 >= 2)
		Reason:    "recent",
	}

	// Cleanup should remove old records
	bn.CleanupReputationRecords()

	// Verify old record is cleaned up, recent record remains
	_, oldExists := bn.reputationRecords[peer.ID("old-peer")]
	assert.False(t, oldExists, "Old record should be cleaned up")

	_, recentExists := bn.reputationRecords[peer.ID("recent-peer")]
	assert.True(t, recentExists, "Recent record should remain")

	// The original peer should still be in the greylist
	status, existsPeer := bn.GetPeerStatus(peerID)
	assert.True(t, existsPeer)
	assert.Equal(t, PeerStatusGreylist, status)
}

func TestBARNetwork_ConcurrentAccess(t *testing.T) {
	bn := NewBARNetwork(nil)

	// Test concurrent access to BAR network
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(id int) {
			peerID := peer.ID(fmt.Sprintf("concurrent-peer-%d", id))
			bn.AddPeer(peerID, fmt.Sprintf("addr%d", id))
			bn.PromoteToWhitelist(peerID)
			bn.UpdatePOMScore(peerID, 1, "concurrent test")
			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify final state is consistent
	whitelist := bn.GetWhitelist()
	greylist := bn.GetGreylist()
	assert.Len(t, whitelist, 8) // Default IncomingConnections
	assert.Len(t, greylist, 2)  // Remaining peers
}

func TestBARNetwork_SeedNodes(t *testing.T) {
	seedNodes := []string{"seed1.example.com:8080", "seed2.example.com:8080"}
	config := &BARConfig{
		SeedNodes: seedNodes,
	}

	bn := NewBARNetwork(config)
	assert.Equal(t, seedNodes, bn.seedNodes)
}

func TestBARNetwork_POMScoreThresholds(t *testing.T) {
	// Test with very low thresholds
	bn := NewBARNetwork(&BARConfig{
		POMThreshold:    1,
		POMBanThreshold: 2,
	})

	peerID := peer.ID("test-peer-1")
	bn.AddPeer(peerID, "addr1")
	bn.PromoteToWhitelist(peerID)

	// Single POM increment should demote
	bn.UpdatePOMScore(peerID, 1, "single increment")
	whitelist := bn.GetWhitelist()
	greylist := bn.GetGreylist()
	assert.Len(t, whitelist, 0)
	assert.Len(t, greylist, 1)

	// Another increment should ban
	bn.UpdatePOMScore(peerID, 1, "ban increment")
	greylist = bn.GetGreylist()
	banned := bn.GetBanned()
	assert.Len(t, greylist, 0)
	assert.Len(t, banned, 1)
}
