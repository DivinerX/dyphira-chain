package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	HandshakeTopic = "/dyphira/handshake/v1"
	PingTimeout    = 5 * time.Second
)

// HandshakeMessage represents a handshake message
type HandshakeMessage struct {
	Type          string  `json:"type"` // "ping", "pong", or "ack"
	From          peer.ID `json:"from"`
	To            peer.ID `json:"to"`
	Round         uint64  `json:"round"`
	RoundSeed     []byte  `json:"round_seed"`
	Timestamp     int64   `json:"timestamp"`
	ClientVersion string  `json:"client_version"`

	// Additional version fields as per BAR specification
	AddrReceived string `json:"addr_received"` // Address where message was received
	AddrFrom     string `json:"addr_from"`     // Address of sender
	LastRound    uint64 `json:"last_round"`    // Last round the node participated in
	Nonce        uint64 `json:"nonce"`         // Random nonce for uniqueness
}

// HandshakeManager manages handshake protocol for BAR network
type HandshakeManager struct {
	node      *AppNode
	round     uint64
	roundSeed []byte
	nonce     uint64 // Current nonce for handshake messages
}

// NewHandshakeManager creates a new handshake manager
func NewHandshakeManager(node *AppNode) *HandshakeManager {
	return &HandshakeManager{
		node:  node,
		nonce: uint64(time.Now().UnixNano()), // Initialize with current timestamp
	}
}

// StartHandshakeManager starts the handshake manager
func (hm *HandshakeManager) Start(ctx context.Context) {
	// Check if P2P node is available (for tests)
	if hm.node.p2p == nil {
		log.Printf("BAR: Handshake manager started without P2P node (test mode)")
		return
	}

	// Register handshake topic
	hm.node.p2p.RegisterTopic(HandshakeTopic)

	// Subscribe to handshake messages
	hm.node.p2p.Subscribe(ctx, hm.handleHandshakeMessage)

	// Start periodic handshake with greylist peers
	go hm.handshakeLoop(ctx)
}

// handleHandshakeMessage handles incoming handshake messages
func (hm *HandshakeManager) handleHandshakeMessage(topic string, msg *pubsub.Message) {
	if topic != HandshakeTopic {
		return
	}

	// Check if P2P node is available (for tests)
	if hm.node.p2p == nil {
		return
	}

	var handshakeMsg HandshakeMessage
	if err := json.Unmarshal(msg.Data, &handshakeMsg); err != nil {
		log.Printf("BAR: Failed to decode handshake message: %v", err)
		return
	}

	// Don't process our own messages
	if handshakeMsg.From == hm.node.p2p.host.ID() {
		return
	}

	switch handshakeMsg.Type {
	case "ping":
		hm.handlePing(&handshakeMsg)
	case "pong":
		hm.handlePong(&handshakeMsg)
	case "ack":
		hm.handleAck(&handshakeMsg)
	}
}

// handlePing handles incoming ping messages
func (hm *HandshakeManager) handlePing(ping *HandshakeMessage) {
	// Check if P2P node is available (for tests)
	if hm.node.p2p == nil {
		return
	}

	// Verify the ping is for us
	if ping.To != hm.node.p2p.host.ID() {
		return
	}

	// Verify round seed is valid for this round
	if !hm.verifyRoundSeed(ping.Round, ping.RoundSeed) {
		log.Printf("BAR: Invalid round seed in ping from %s", ping.From)
		hm.node.barNet.UpdatePOMScore(ping.From, 1, "invalid round seed")
		return
	}

	// Verify that this peer is actually selected by our PRNG for this round
	selectedPeers := hm.node.barNet.FindNodes(ping.Round)
	isSelected := false
	for _, selectedPeer := range selectedPeers {
		if selectedPeer == ping.From {
			isSelected = true
			break
		}
	}

	if !isSelected {
		log.Printf("BAR: Peer %s not selected by PRNG for round %d", ping.From, ping.Round)
		hm.node.barNet.UpdatePOMScore(ping.From, 1, "not selected by PRNG")
		return
	}

	// Send pong response with full version info
	pong := &HandshakeMessage{
		Type:          "pong",
		From:          hm.node.p2p.host.ID(),
		To:            ping.From,
		Round:         ping.Round,
		RoundSeed:     ping.RoundSeed,
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "dyphira-v1.0",
		AddrReceived:  ping.AddrFrom,
		AddrFrom:      hm.node.p2p.GetListenAddr(),
		LastRound:     hm.round,
		Nonce:         hm.generateNonce(),
	}

	pongData, err := json.Marshal(pong)
	if err != nil {
		log.Printf("BAR: Failed to marshal pong: %v", err)
		return
	}

	if err := hm.node.p2p.Publish(hm.node.ctx, HandshakeTopic, pongData); err != nil {
		log.Printf("BAR: Failed to send pong: %v", err)
		return
	}

	// Send ACK message to confirm successful handshake
	ack := &HandshakeMessage{
		Type:          "ack",
		From:          hm.node.p2p.host.ID(),
		To:            ping.From,
		Round:         ping.Round,
		RoundSeed:     ping.RoundSeed,
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "dyphira-v1.0",
		AddrReceived:  ping.AddrFrom,
		AddrFrom:      hm.node.p2p.GetListenAddr(),
		LastRound:     hm.round,
		Nonce:         hm.generateNonce(),
	}

	ackData, err := json.Marshal(ack)
	if err != nil {
		log.Printf("BAR: Failed to marshal ack: %v", err)
		return
	}

	if err := hm.node.p2p.Publish(hm.node.ctx, HandshakeTopic, ackData); err != nil {
		log.Printf("BAR: Failed to send ack: %v", err)
	}

	log.Printf("BAR: Sent pong and ack to peer %s for round %d", ping.From, ping.Round)
}

// handlePong handles incoming pong messages
func (hm *HandshakeManager) handlePong(pong *HandshakeMessage) {
	// Check if P2P node is available (for tests)
	if hm.node.p2p == nil {
		return
	}

	// Verify the pong is for us
	if pong.To != hm.node.p2p.host.ID() {
		return
	}

	// Verify round seed is valid
	if !hm.verifyRoundSeed(pong.Round, pong.RoundSeed) {
		log.Printf("BAR: Invalid round seed in pong from %s", pong.From)
		hm.node.barNet.UpdatePOMScore(pong.From, 1, "invalid round seed")
		return
	}

	// Validate version information
	if pong.ClientVersion == "" {
		log.Printf("BAR: Missing client version in pong from %s", pong.From)
		hm.node.barNet.UpdatePOMScore(pong.From, 1, "missing client version")
		return
	}

	// Check if this peer is in our greylist
	status, exists := hm.node.barNet.GetPeerStatus(pong.From)
	if !exists || status != PeerStatusGreylist {
		return
	}

	// Promote to whitelist on successful handshake
	if hm.node.barNet.PromoteToWhitelist(pong.From) {
		log.Printf("BAR: Promoted peer %s to whitelist after successful handshake", pong.From)

		// Notify P2P layer of successful handshake
		if hm.node.p2p.OnPeerHandshake != nil {
			hm.node.p2p.OnPeerHandshake(pong.From, pong.AddrFrom)
		}
	}
}

// handleAck handles incoming ack messages
func (hm *HandshakeManager) handleAck(ack *HandshakeMessage) {
	// Check if P2P node is available (for tests)
	if hm.node.p2p == nil {
		return
	}

	// Verify the ack is for us
	if ack.To != hm.node.p2p.host.ID() {
		return
	}

	// Verify round seed is valid
	if !hm.verifyRoundSeed(ack.Round, ack.RoundSeed) {
		log.Printf("BAR: Invalid round seed in ack from %s", ack.From)
		hm.node.barNet.UpdatePOMScore(ack.From, 1, "invalid round seed")
		return
	}

	// Validate version information
	if ack.ClientVersion == "" {
		log.Printf("BAR: Missing client version in ack from %s", ack.From)
		hm.node.barNet.UpdatePOMScore(ack.From, 1, "missing client version")
		return
	}

	log.Printf("BAR: Received ack from peer %s for round %d", ack.From, ack.Round)
}

// SendPing sends a ping to a specific peer
func (hm *HandshakeManager) SendPing(peerID peer.ID) error {
	// Check if P2P node is available (for tests)
	if hm.node.p2p == nil {
		return fmt.Errorf("P2P node not available")
	}

	ping := &HandshakeMessage{
		Type:          "ping",
		From:          hm.node.p2p.host.ID(),
		To:            peerID,
		Round:         hm.round,
		RoundSeed:     hm.roundSeed,
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "dyphira-v1.0",
		AddrReceived:  "", // Will be filled by recipient
		AddrFrom:      hm.node.p2p.GetListenAddr(),
		LastRound:     hm.round,
		Nonce:         hm.generateNonce(),
	}

	pingData, err := json.Marshal(ping)
	if err != nil {
		return fmt.Errorf("failed to marshal ping: %v", err)
	}

	return hm.node.p2p.Publish(hm.node.ctx, HandshakeTopic, pingData)
}

// handshakeLoop periodically performs handshakes with greylist peers
func (hm *HandshakeManager) handshakeLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second) // Handshake every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			hm.performHandshakes()
		}
	}
}

// performHandshakes performs handshakes with greylist peers
func (hm *HandshakeManager) performHandshakes() {
	greylist := hm.node.barNet.GetGreylist()

	// Limit concurrent handshakes to avoid spam
	maxHandshakes := 3
	handshakeCount := 0

	for _, peerInfo := range greylist {
		if handshakeCount >= maxHandshakes {
			break
		}

		// Skip if peer was seen recently
		if time.Since(peerInfo.LastSeen) < 10*time.Second {
			continue
		}

		// Send ping
		if err := hm.SendPing(peerInfo.ID); err != nil {
			log.Printf("BAR: Failed to send ping to %s: %v", peerInfo.ID, err)
			hm.node.barNet.UpdatePOMScore(peerInfo.ID, 1, "failed to send ping")
		} else {
			handshakeCount++
		}
	}
}

// UpdateRound updates the current round and generates new round seed
func (hm *HandshakeManager) UpdateRound(round uint64) {
	hm.round = round
	hm.roundSeed = hm.node.barNet.generateRoundSeed(round)
	log.Printf("BAR: Updated handshake round to %d", round)
}

// verifyRoundSeed verifies if a round seed is valid for a given round
func (hm *HandshakeManager) verifyRoundSeed(round uint64, seed []byte) bool {
	expectedSeed := hm.node.barNet.generateRoundSeed(round)
	return len(seed) == len(expectedSeed) &&
		len(seed) > 0 &&
		seed[0] == expectedSeed[0] // Simple check for now
}

// GetRound returns the current round
func (hm *HandshakeManager) GetRound() uint64 {
	return hm.round
}

// generateNonce generates a new nonce for handshake messages
func (hm *HandshakeManager) generateNonce() uint64 {
	hm.nonce++
	return hm.nonce
}
