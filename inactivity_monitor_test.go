package main

import (
	"context"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInactivityMonitor_NewInactivityMonitor(t *testing.T) {
	node := &AppNode{
		ctx: context.Background(),
	}

	// Test regular node
	im := NewInactivityMonitor(node, false)
	require.NotNil(t, im)
	assert.Equal(t, node, im.node)
	assert.False(t, im.isSeedNode)
	assert.Equal(t, 3, im.seedNodeThreshold)

	// Test seed node
	imSeed := NewInactivityMonitor(node, true)
	require.NotNil(t, imSeed)
	assert.True(t, imSeed.isSeedNode)
}

func TestInactivityMessage_Structure(t *testing.T) {
	// Create proper peer IDs for testing
	peer1 := peer.ID("test-peer-1")
	peer2 := peer.ID("test-peer-2")
	targetPeer := peer.ID("target-peer")

	msg := &InactivityMessage{
		Type:          "mark_inactive",
		From:          peer1,
		To:            peer2,
		TargetPeer:    targetPeer,
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "test-v1.0",
		Reason:        "no recent activity",
		Evidence:      []string{"timeout", "no handshake"},
	}

	// Test that all fields are properly set
	assert.Equal(t, "mark_inactive", msg.Type)
	assert.Equal(t, peer1, msg.From)
	assert.Equal(t, peer2, msg.To)
	assert.Equal(t, targetPeer, msg.TargetPeer)
	assert.Equal(t, "test-v1.0", msg.ClientVersion)
	assert.Equal(t, "no recent activity", msg.Reason)
	assert.Len(t, msg.Evidence, 2)
	assert.Contains(t, msg.Evidence, "timeout")
	assert.Contains(t, msg.Evidence, "no handshake")
}

func TestInactivityRecord_Structure(t *testing.T) {
	// Create proper peer IDs for testing
	peer1 := peer.ID("test-peer-1")
	reporter1 := peer.ID("reporter-1")
	reporter2 := peer.ID("reporter-2")

	now := time.Now()
	record := &InactivityRecord{
		PeerID:        peer1,
		FirstReported: now,
		LastReported:  now,
		ReportCount:   2,
		Reporters:     []peer.ID{reporter1, reporter2},
		Reasons:       []string{"timeout", "no handshake"},
		Evidence:      []string{"evidence1", "evidence2"},
	}

	// Test that all fields are properly set
	assert.Equal(t, peer1, record.PeerID)
	assert.Equal(t, now, record.FirstReported)
	assert.Equal(t, now, record.LastReported)
	assert.Equal(t, 2, record.ReportCount)
	assert.Len(t, record.Reporters, 2)
	assert.Len(t, record.Reasons, 2)
	assert.Len(t, record.Evidence, 2)
}

func TestInactivityMonitor_RecordInactivity(t *testing.T) {
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	im := NewInactivityMonitor(node, true) // Seed node

	// Create test peer IDs
	peer1 := peer.ID("test-peer-1")
	reporter1 := peer.ID("reporter-1")
	reporter2 := peer.ID("reporter-2")

	// Record first inactivity report
	im.recordInactivity(peer1, reporter1, "timeout", []string{"evidence1"})

	// Check record was created
	record, exists := im.GetInactivityRecord(peer1)
	require.True(t, exists)
	assert.Equal(t, peer1, record.PeerID)
	assert.Equal(t, 1, record.ReportCount)
	assert.Len(t, record.Reporters, 1)
	assert.Equal(t, reporter1, record.Reporters[0])

	// Record second inactivity report from different reporter
	im.recordInactivity(peer1, reporter2, "no handshake", []string{"evidence2"})

	// Check record was updated
	record, exists = im.GetInactivityRecord(peer1)
	require.True(t, exists)
	assert.Equal(t, 2, record.ReportCount)
	assert.Len(t, record.Reporters, 2)
	assert.Contains(t, record.Reporters, reporter1)
	assert.Contains(t, record.Reporters, reporter2)
	assert.Len(t, record.Reasons, 2)
	assert.Contains(t, record.Reasons, "timeout")
	assert.Contains(t, record.Reasons, "no handshake")
}

func TestInactivityMonitor_ShouldMarkAsInactive(t *testing.T) {
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	im := NewInactivityMonitor(node, true) // Seed node

	// Create test peer IDs
	peer1 := peer.ID("test-peer-1")
	reporter1 := peer.ID("reporter-1")
	reporter2 := peer.ID("reporter-2")
	reporter3 := peer.ID("reporter-3")

	// Test with no record
	assert.False(t, im.shouldMarkAsInactive(peer1))

	// Test with insufficient reports
	im.recordInactivity(peer1, reporter1, "timeout", []string{"evidence1"})
	assert.False(t, im.shouldMarkAsInactive(peer1))

	// Test with enough seed node reports (threshold = 3)
	im.recordInactivity(peer1, reporter2, "no handshake", []string{"evidence2"})
	im.recordInactivity(peer1, reporter3, "no response", []string{"evidence3"})
	assert.True(t, im.shouldMarkAsInactive(peer1))
}

func TestInactivityMonitor_GetAllInactivityRecords(t *testing.T) {
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	im := NewInactivityMonitor(node, true) // Seed node

	// Create test peer IDs
	peer1 := peer.ID("test-peer-1")
	peer2 := peer.ID("test-peer-2")
	reporter := peer.ID("reporter-1")

	// Add records
	im.recordInactivity(peer1, reporter, "timeout", []string{"evidence1"})
	im.recordInactivity(peer2, reporter, "no handshake", []string{"evidence2"})

	// Get all records
	records := im.GetAllInactivityRecords()
	assert.Len(t, records, 2)
	assert.Contains(t, records, peer1)
	assert.Contains(t, records, peer2)
}

func TestInactivityMonitor_ClearInactivityRecord(t *testing.T) {
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	im := NewInactivityMonitor(node, true) // Seed node

	// Create test peer IDs
	peer1 := peer.ID("test-peer-1")
	reporter := peer.ID("reporter-1")

	// Add record
	im.recordInactivity(peer1, reporter, "timeout", []string{"evidence1"})

	// Verify record exists
	_, exists := im.GetInactivityRecord(peer1)
	assert.True(t, exists)

	// Clear record
	im.ClearInactivityRecord(peer1)

	// Verify record is gone
	_, exists = im.GetInactivityRecord(peer1)
	assert.False(t, exists)
}

func TestInactivityMonitor_MessageTypes(t *testing.T) {
	// Create proper peer IDs for testing
	peer1 := peer.ID("peer1")
	peer2 := peer.ID("peer2")
	targetPeer := peer.ID("target")

	// Test mark_inactive message
	markMsg := &InactivityMessage{
		Type:          "mark_inactive",
		From:          peer1,
		To:            peer2,
		TargetPeer:    targetPeer,
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "v1.0",
		Reason:        "timeout",
		Evidence:      []string{"evidence1"},
	}

	// Test inactivity_report message
	reportMsg := &InactivityMessage{
		Type:          "inactivity_report",
		From:          peer1,
		To:            peer2,
		TargetPeer:    targetPeer,
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "v1.0",
		Reason:        "marked inactive",
		Evidence:      []string{"evidence1", "evidence2"},
	}

	// Verify message types
	assert.Equal(t, "mark_inactive", markMsg.Type)
	assert.Equal(t, "inactivity_report", reportMsg.Type)
}

func TestInactivityMonitor_SeedNodeBehavior(t *testing.T) {
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	// Test regular node
	imRegular := NewInactivityMonitor(node, false)
	assert.False(t, imRegular.isSeedNode)

	// Test seed node
	imSeed := NewInactivityMonitor(node, true)
	assert.True(t, imSeed.isSeedNode)

	// Test that seed nodes have different behavior
	// (This would be tested in integration tests with actual P2P communication)
}

func TestInactivityMonitor_Integration(t *testing.T) {
	// Create a node with BAR network
	ctx := context.Background()

	node := &AppNode{
		ctx:    ctx,
		barNet: NewBARNetwork(nil),
	}

	im := NewInactivityMonitor(node, true) // Seed node

	// Test that the monitor can be started and stopped
	im.Start(ctx)

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Stop the monitor
	im.Stop()

	// Test that the monitor can access BAR network methods
	allPeers := im.node.barNet.GetAllPeers()
	assert.NotNil(t, allPeers) // Should be empty but not nil
}
