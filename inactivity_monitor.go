package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	InactivityTopic         = "/dyphira/inactivity/v1"
	InactivityTimeout       = 5 * time.Minute // Time after which a peer is considered inactive
	InactivityCheckInterval = 1 * time.Minute // How often to check for inactivity
)

// InactivityMessage represents an inactivity marking message
type InactivityMessage struct {
	Type          string   `json:"type"` // "mark_inactive", "inactivity_report"
	From          peer.ID  `json:"from"`
	To            peer.ID  `json:"to"`
	TargetPeer    peer.ID  `json:"target_peer"`
	Timestamp     int64    `json:"timestamp"`
	ClientVersion string   `json:"client_version"`
	Reason        string   `json:"reason"`
	Evidence      []string `json:"evidence"` // List of evidence for inactivity
}

// InactivityRecord tracks inactivity for a specific peer
type InactivityRecord struct {
	PeerID        peer.ID   `json:"peer_id"`
	FirstReported time.Time `json:"first_reported"`
	LastReported  time.Time `json:"last_reported"`
	ReportCount   int       `json:"report_count"`
	Reporters     []peer.ID `json:"reporters"`
	Reasons       []string  `json:"reasons"`
	Evidence      []string  `json:"evidence"`
}

// InactivityMonitor manages inactivity monitoring for seed nodes
type InactivityMonitor struct {
	node *AppNode

	// Inactivity records
	records   map[peer.ID]*InactivityRecord
	recordsMu sync.RWMutex

	// Seed node status
	isSeedNode        bool
	seedNodeThreshold int // Minimum number of reports to mark as inactive

	// Monitoring state
	stopChan chan struct{}
	wg       sync.WaitGroup
}

// NewInactivityMonitor creates a new inactivity monitor
func NewInactivityMonitor(node *AppNode, isSeedNode bool) *InactivityMonitor {
	return &InactivityMonitor{
		node:              node,
		records:           make(map[peer.ID]*InactivityRecord),
		isSeedNode:        isSeedNode,
		seedNodeThreshold: 3, // Require 3 seed nodes to report inactivity
		stopChan:          make(chan struct{}),
	}
}

// StartInactivityMonitor starts the inactivity monitoring
func (im *InactivityMonitor) Start(ctx context.Context) {
	// Check if P2P node is available (for tests)
	if im.node.p2p == nil {
		log.Printf("BAR: Inactivity monitor started without P2P node (test mode)")
		return
	}

	// Register inactivity topic
	im.node.p2p.RegisterTopic(InactivityTopic)

	// Subscribe to inactivity messages
	im.node.p2p.Subscribe(ctx, im.handleInactivityMessage)

	// Start monitoring loop if this is a seed node
	if im.isSeedNode {
		im.wg.Add(1)
		go im.monitoringLoop(ctx)
	}

	log.Printf("BAR: Inactivity monitor started (seed node: %v)", im.isSeedNode)
}

// Stop stops the inactivity monitor
func (im *InactivityMonitor) Stop() {
	close(im.stopChan)
	im.wg.Wait()
	log.Printf("BAR: Inactivity monitor stopped")
}

// monitoringLoop runs the main monitoring loop for seed nodes
func (im *InactivityMonitor) monitoringLoop(ctx context.Context) {
	defer im.wg.Done()

	ticker := time.NewTicker(InactivityCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-im.stopChan:
			return
		case <-ticker.C:
			im.checkForInactivity()
		}
	}
}

// checkForInactivity checks all peers for inactivity
func (im *InactivityMonitor) checkForInactivity() {
	// Check if P2P node is available (for tests)
	if im.node.p2p == nil {
		return
	}

	// Get all peers from BAR network
	allPeers := im.node.barNet.GetAllPeers()

	for _, peerInfo := range allPeers {
		// Skip ourselves
		if peerInfo.ID == im.node.p2p.host.ID() {
			continue
		}

		// Check if peer is inactive
		if im.isPeerInactive(peerInfo.ID) {
			im.reportInactivity(peerInfo.ID, "no recent activity", []string{"timeout"})
		}
	}
}

// isPeerInactive checks if a peer is inactive
func (im *InactivityMonitor) isPeerInactive(peerID peer.ID) bool {
	// Check if peer has been active recently
	// This would typically check:
	// 1. Last message received from peer
	// 2. Last handshake with peer
	// 3. Last block/transaction from peer

	// For now, we'll use a simple timeout-based approach
	// In a real implementation, this would check actual activity timestamps

	// Check if peer is in our whitelist (active peers)
	status, exists := im.node.barNet.GetPeerStatus(peerID)
	if !exists || status == PeerStatusBanned {
		return false // Don't report banned peers as inactive
	}

	// Simple heuristic: if peer is in greylist for too long, consider it inactive
	if status == PeerStatusGreylist {
		// Check how long peer has been in greylist
		// This would require tracking when peer was added to greylist
		return true
	}

	return false
}

// reportInactivity reports a peer as inactive to other seed nodes
func (im *InactivityMonitor) reportInactivity(peerID peer.ID, reason string, evidence []string) {
	if !im.isSeedNode {
		return // Only seed nodes can report inactivity
	}

	// Check if P2P node is available (for tests)
	if im.node.p2p == nil {
		return
	}

	// Get other seed nodes (whitelisted peers)
	whitelist := im.node.barNet.GetWhitelist()

	for _, seedNode := range whitelist {
		if seedNode.ID == im.node.p2p.host.ID() {
			continue // Don't report to ourselves
		}

		report := &InactivityMessage{
			Type:          "mark_inactive",
			From:          im.node.p2p.host.ID(),
			To:            seedNode.ID,
			TargetPeer:    peerID,
			Timestamp:     time.Now().UnixNano(),
			ClientVersion: "dyphira-v1.0",
			Reason:        reason,
			Evidence:      evidence,
		}

		reportData, err := json.Marshal(report)
		if err != nil {
			log.Printf("BAR: Failed to marshal inactivity report: %v", err)
			continue
		}

		if err := im.node.p2p.Publish(im.node.ctx, InactivityTopic, reportData); err != nil {
			log.Printf("BAR: Failed to send inactivity report to %s: %v", seedNode.ID, err)
			continue
		}

		log.Printf("BAR: Sent inactivity report for peer %s to seed node %s", peerID, seedNode.ID)
	}
}

// handleInactivityMessage handles incoming inactivity messages
func (im *InactivityMonitor) handleInactivityMessage(topic string, msg *pubsub.Message) {
	if topic != InactivityTopic {
		return
	}

	// Check if P2P node is available (for tests)
	if im.node.p2p == nil {
		return
	}

	var inactivityMsg InactivityMessage
	if err := json.Unmarshal(msg.Data, &inactivityMsg); err != nil {
		log.Printf("BAR: Failed to decode inactivity message: %v", err)
		im.node.barNet.UpdatePOMScore(msg.ReceivedFrom, 1, "malformed inactivity message")
		return
	}

	// Don't process our own messages
	if inactivityMsg.From == im.node.p2p.host.ID() {
		return
	}

	switch inactivityMsg.Type {
	case "mark_inactive":
		im.handleMarkInactive(&inactivityMsg)
	case "inactivity_report":
		im.handleInactivityReport(&inactivityMsg)
	}
}

// handleMarkInactive handles incoming inactivity marking requests
func (im *InactivityMonitor) handleMarkInactive(markMsg *InactivityMessage) {
	// Check if P2P node is available (for tests)
	if im.node.p2p == nil {
		return
	}

	// Verify the message is for us
	if markMsg.To != im.node.p2p.host.ID() {
		return
	}

	// Only seed nodes should receive these messages
	if !im.isSeedNode {
		log.Printf("BAR: Received inactivity mark from non-seed node %s", markMsg.From)
		im.node.barNet.UpdatePOMScore(markMsg.From, 1, "non-seed node sending inactivity mark")
		return
	}

	log.Printf("BAR: Received inactivity mark for peer %s from seed node %s",
		markMsg.TargetPeer, markMsg.From)

	// Record the inactivity report
	im.recordInactivity(markMsg.TargetPeer, markMsg.From, markMsg.Reason, markMsg.Evidence)

	// Check if we should mark the peer as inactive
	if im.shouldMarkAsInactive(markMsg.TargetPeer) {
		im.markPeerAsInactive(markMsg.TargetPeer)
	}
}

// handleInactivityReport handles incoming inactivity reports
func (im *InactivityMonitor) handleInactivityReport(reportMsg *InactivityMessage) {
	// Check if P2P node is available (for tests)
	if im.node.p2p == nil {
		return
	}

	// Verify the message is for us
	if reportMsg.To != im.node.p2p.host.ID() {
		return
	}

	log.Printf("BAR: Received inactivity report for peer %s from %s",
		reportMsg.TargetPeer, reportMsg.From)

	// Process the report
	im.recordInactivity(reportMsg.TargetPeer, reportMsg.From, reportMsg.Reason, reportMsg.Evidence)
}

// recordInactivity records an inactivity report for a peer
func (im *InactivityMonitor) recordInactivity(peerID, reporter peer.ID, reason string, evidence []string) {
	im.recordsMu.Lock()
	defer im.recordsMu.Unlock()

	now := time.Now()

	record, exists := im.records[peerID]
	if !exists {
		record = &InactivityRecord{
			PeerID:        peerID,
			FirstReported: now,
			LastReported:  now,
			ReportCount:   0,
			Reporters:     make([]peer.ID, 0),
			Reasons:       make([]string, 0),
			Evidence:      make([]string, 0),
		}
		im.records[peerID] = record
	}

	// Update record
	record.LastReported = now
	record.ReportCount++

	// Add reporter if not already present
	found := false
	for _, existingReporter := range record.Reporters {
		if existingReporter == reporter {
			found = true
			break
		}
	}
	if !found {
		record.Reporters = append(record.Reporters, reporter)
	}

	// Add reason and evidence
	record.Reasons = append(record.Reasons, reason)
	record.Evidence = append(record.Evidence, evidence...)

	log.Printf("BAR: Recorded inactivity for peer %s (reports: %d, reporters: %d)",
		peerID, record.ReportCount, len(record.Reporters))
}

// shouldMarkAsInactive determines if a peer should be marked as inactive
func (im *InactivityMonitor) shouldMarkAsInactive(peerID peer.ID) bool {
	im.recordsMu.RLock()
	defer im.recordsMu.RUnlock()

	record, exists := im.records[peerID]
	if !exists {
		return false
	}

	// Check if enough seed nodes have reported inactivity
	if len(record.Reporters) >= im.seedNodeThreshold {
		return true
	}

	// Check if peer has been reported multiple times over a long period
	if record.ReportCount >= 5 && time.Since(record.FirstReported) > 10*time.Minute {
		return true
	}

	return false
}

// markPeerAsInactive marks a peer as inactive in the BAR network
func (im *InactivityMonitor) markPeerAsInactive(peerID peer.ID) {
	log.Printf("BAR: Marking peer %s as inactive", peerID)

	// Move peer to greylist or ban list based on severity
	// For now, we'll move to greylist
	im.node.barNet.DemoteToGreylist(peerID)

	// Send inactivity report to other nodes
	im.broadcastInactivityReport(peerID)
}

// broadcastInactivityReport broadcasts an inactivity report to all peers
func (im *InactivityMonitor) broadcastInactivityReport(peerID peer.ID) {
	im.recordsMu.RLock()
	record, exists := im.records[peerID]
	im.recordsMu.RUnlock()

	if !exists {
		return
	}

	report := &InactivityMessage{
		Type:          "inactivity_report",
		From:          im.node.p2p.host.ID(),
		To:            "", // Broadcast to all
		TargetPeer:    peerID,
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "dyphira-v1.0",
		Reason:        fmt.Sprintf("marked inactive by %d seed nodes", len(record.Reporters)),
		Evidence:      record.Evidence,
	}

	reportData, err := json.Marshal(report)
	if err != nil {
		log.Printf("BAR: Failed to marshal inactivity report: %v", err)
		return
	}

	if err := im.node.p2p.Publish(im.node.ctx, InactivityTopic, reportData); err != nil {
		log.Printf("BAR: Failed to broadcast inactivity report: %v", err)
		return
	}

	log.Printf("BAR: Broadcasted inactivity report for peer %s", peerID)
}

// GetInactivityRecord returns the inactivity record for a peer
func (im *InactivityMonitor) GetInactivityRecord(peerID peer.ID) (*InactivityRecord, bool) {
	im.recordsMu.RLock()
	defer im.recordsMu.RUnlock()

	record, exists := im.records[peerID]
	return record, exists
}

// GetAllInactivityRecords returns all inactivity records
func (im *InactivityMonitor) GetAllInactivityRecords() map[peer.ID]*InactivityRecord {
	im.recordsMu.RLock()
	defer im.recordsMu.RUnlock()

	// Return a copy to prevent external modifications
	recordsCopy := make(map[peer.ID]*InactivityRecord)
	for k, v := range im.records {
		recordsCopy[k] = v
	}
	return recordsCopy
}

// ClearInactivityRecord clears the inactivity record for a peer
func (im *InactivityMonitor) ClearInactivityRecord(peerID peer.ID) {
	im.recordsMu.Lock()
	defer im.recordsMu.Unlock()

	delete(im.records, peerID)
	log.Printf("BAR: Cleared inactivity record for peer %s", peerID)
}
