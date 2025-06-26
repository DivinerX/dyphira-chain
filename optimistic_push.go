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
	OptimisticPushTopic = "/dyphira/optimistic-push/v1"
	PushTimeout         = 30 * time.Second
)

// OptimisticPushMessage represents an optimistic push message
type OptimisticPushMessage struct {
	Type          string  `json:"type"` // "push_request", "push_response", "block_data"
	From          peer.ID `json:"from"`
	To            peer.ID `json:"to"`
	RequestID     string  `json:"request_id"`
	Timestamp     int64   `json:"timestamp"`
	ClientVersion string  `json:"client_version"`

	// Block data for push response
	BlockHeight  uint64        `json:"block_height,omitempty"`
	BlockData    interface{}   `json:"block_data,omitempty"`
	BlockHeaders []interface{} `json:"block_headers,omitempty"` // For light nodes
	IsLightNode  bool          `json:"is_light_node"`
}

// OptimisticPushManager manages optimistic push protocol for new nodes
type OptimisticPushManager struct {
	node *AppNode
}

// NewOptimisticPushManager creates a new optimistic push manager
func NewOptimisticPushManager(node *AppNode) *OptimisticPushManager {
	return &OptimisticPushManager{
		node: node,
	}
}

// StartOptimisticPushManager starts the optimistic push manager
func (opm *OptimisticPushManager) Start(ctx context.Context) {
	// Register optimistic push topic
	opm.node.p2p.RegisterTopic(OptimisticPushTopic)

	// Subscribe to optimistic push messages
	opm.node.p2p.Subscribe(ctx, opm.handleOptimisticPushMessage)

	log.Printf("BAR: Optimistic push manager started")
}

// handleOptimisticPushMessage handles incoming optimistic push messages
func (opm *OptimisticPushManager) handleOptimisticPushMessage(topic string, msg *pubsub.Message) {
	if topic != OptimisticPushTopic {
		return
	}

	var pushMsg OptimisticPushMessage
	if err := json.Unmarshal(msg.Data, &pushMsg); err != nil {
		log.Printf("BAR: Failed to decode optimistic push message: %v", err)
		opm.node.barNet.UpdatePOMScore(msg.ReceivedFrom, 1, "malformed optimistic push message")
		return
	}

	// Don't process our own messages
	if pushMsg.From == opm.node.p2p.host.ID() {
		return
	}

	switch pushMsg.Type {
	case "push_request":
		opm.handlePushRequest(&pushMsg)
	case "push_response":
		opm.handlePushResponse(&pushMsg)
	case "block_data":
		opm.handleBlockData(&pushMsg)
	}
}

// handlePushRequest handles incoming push requests from new nodes
func (opm *OptimisticPushManager) handlePushRequest(request *OptimisticPushMessage) {
	// Verify the request is for us
	if request.To != opm.node.p2p.host.ID() {
		return
	}

	// Check if this peer is in our whitelist (only altruistic/rational nodes should respond)
	status, exists := opm.node.barNet.GetPeerStatus(request.From)
	if !exists || status != PeerStatusWhitelist {
		log.Printf("BAR: Ignoring push request from non-whitelisted peer %s", request.From)
		return
	}

	log.Printf("BAR: Received push request from peer %s", request.From)

	// Send push response with block data
	opm.sendPushResponse(request.From, request.RequestID, request.IsLightNode)
}

// handlePushResponse handles incoming push responses
func (opm *OptimisticPushManager) handlePushResponse(response *OptimisticPushMessage) {
	// Verify the response is for us
	if response.To != opm.node.p2p.host.ID() {
		return
	}

	log.Printf("BAR: Received push response from peer %s for request %s",
		response.From, response.RequestID)

	// Process the received block data
	if response.BlockData != nil {
		opm.processReceivedBlockData(response.BlockData, response.BlockHeight)
	}

	if response.BlockHeaders != nil {
		opm.processReceivedBlockHeaders(response.BlockHeaders)
	}
}

// handleBlockData handles incoming block data messages
func (opm *OptimisticPushManager) handleBlockData(blockMsg *OptimisticPushMessage) {
	// Verify the block data is for us
	if blockMsg.To != opm.node.p2p.host.ID() {
		return
	}

	log.Printf("BAR: Received block data from peer %s for height %d",
		blockMsg.From, blockMsg.BlockHeight)

	// Process the received block data
	if blockMsg.BlockData != nil {
		opm.processReceivedBlockData(blockMsg.BlockData, blockMsg.BlockHeight)
	}
}

// sendPushResponse sends a push response with block data to a requesting peer
func (opm *OptimisticPushManager) sendPushResponse(to peer.ID, requestID string, isLightNode bool) {
	// Get current blockchain state
	currentHeight := opm.node.bc.Height()

	response := &OptimisticPushMessage{
		Type:          "push_response",
		From:          opm.node.p2p.host.ID(),
		To:            to,
		RequestID:     requestID,
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "dyphira-v1.0",
		BlockHeight:   currentHeight,
		IsLightNode:   isLightNode,
	}

	// Add block data based on node type
	if isLightNode {
		// For light nodes, send block headers
		response.BlockHeaders = opm.getBlockHeaders(currentHeight)
	} else {
		// For full nodes, send full block data
		response.BlockData = opm.getBlockData(currentHeight)
	}

	responseData, err := json.Marshal(response)
	if err != nil {
		log.Printf("BAR: Failed to marshal push response: %v", err)
		return
	}

	if err := opm.node.p2p.Publish(opm.node.ctx, OptimisticPushTopic, responseData); err != nil {
		log.Printf("BAR: Failed to send push response: %v", err)
		return
	}

	log.Printf("BAR: Sent push response to peer %s with %d blocks", to, currentHeight)
}

// RequestOptimisticPush requests optimistic push from whitelisted peers
func (opm *OptimisticPushManager) RequestOptimisticPush(isLightNode bool) error {
	// Check if P2P component is initialized
	if opm.node.p2p == nil {
		return fmt.Errorf("P2P component not initialized")
	}

	// Get whitelisted peers
	whitelist := opm.node.barNet.GetWhitelist()
	if len(whitelist) == 0 {
		return fmt.Errorf("no whitelisted peers available for optimistic push")
	}

	requestID := fmt.Sprintf("push_%d", time.Now().UnixNano())

	// Send push request to all whitelisted peers
	for _, peerInfo := range whitelist {
		request := &OptimisticPushMessage{
			Type:          "push_request",
			From:          opm.node.p2p.host.ID(),
			To:            peerInfo.ID,
			RequestID:     requestID,
			Timestamp:     time.Now().UnixNano(),
			ClientVersion: "dyphira-v1.0",
			IsLightNode:   isLightNode,
		}

		requestData, err := json.Marshal(request)
		if err != nil {
			log.Printf("BAR: Failed to marshal push request: %v", err)
			continue
		}

		if err := opm.node.p2p.Publish(opm.node.ctx, OptimisticPushTopic, requestData); err != nil {
			log.Printf("BAR: Failed to send push request to %s: %v", peerInfo.ID, err)
			continue
		}

		log.Printf("BAR: Sent push request to peer %s", peerInfo.ID)
	}

	return nil
}

// getBlockHeaders returns block headers for light nodes
func (opm *OptimisticPushManager) getBlockHeaders(height uint64) []interface{} {
	headers := make([]interface{}, 0)

	// Get headers for the last 100 blocks (or all if less)
	startHeight := uint64(0)
	if height > 100 {
		startHeight = height - 100
	}

	for i := startHeight; i <= height; i++ {
		block, err := opm.node.bc.GetBlockByHeight(i)
		if err != nil {
			continue
		}
		headers = append(headers, block.Header)
	}

	return headers
}

// getBlockData returns full block data for full nodes
func (opm *OptimisticPushManager) getBlockData(height uint64) interface{} {
	// Return the latest block for now
	block, err := opm.node.bc.GetBlockByHeight(height)
	if err != nil {
		return nil
	}
	return block
}

// processReceivedBlockData processes received block data
func (opm *OptimisticPushManager) processReceivedBlockData(blockData interface{}, height uint64) {
	log.Printf("BAR: Processing received block data for height %d", height)

	// TODO: Implement block data processing
	// This would involve:
	// 1. Validating the block data
	// 2. Adding blocks to the blockchain if they're missing
	// 3. Updating the node's state
}

// processReceivedBlockHeaders processes received block headers
func (opm *OptimisticPushManager) processReceivedBlockHeaders(headers []interface{}) {
	log.Printf("BAR: Processing received %d block headers", len(headers))

	// TODO: Implement block header processing for light nodes
	// This would involve:
	// 1. Validating the headers
	// 2. Storing headers for light node verification
	// 3. Requesting full blocks for headers of interest
}

// TriggerOptimisticPushForNewPeer triggers optimistic push for a newly promoted peer
func (opm *OptimisticPushManager) TriggerOptimisticPushForNewPeer(peerID peer.ID) {
	// Check if P2P component is initialized
	if opm.node.p2p == nil {
		log.Printf("BAR: Cannot trigger optimistic push - P2P component not initialized")
		return
	}

	log.Printf("BAR: Triggering optimistic push for newly promoted peer %s", peerID)

	// Send a push request to the new peer
	request := &OptimisticPushMessage{
		Type:          "push_request",
		From:          opm.node.p2p.host.ID(),
		To:            peerID,
		RequestID:     fmt.Sprintf("new_peer_push_%d", time.Now().UnixNano()),
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "dyphira-v1.0",
		IsLightNode:   false, // Assume full node for now
	}

	requestData, err := json.Marshal(request)
	if err != nil {
		log.Printf("BAR: Failed to marshal new peer push request: %v", err)
		return
	}

	if err := opm.node.p2p.Publish(opm.node.ctx, OptimisticPushTopic, requestData); err != nil {
		log.Printf("BAR: Failed to send new peer push request: %v", err)
		return
	}

	log.Printf("BAR: Sent optimistic push request to new peer %s", peerID)
}
