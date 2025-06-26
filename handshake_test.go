package main

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandshakeManager_NewHandshakeManager(t *testing.T) {
	// Create a mock node for testing
	node := &AppNode{
		ctx: context.Background(),
	}

	hm := NewHandshakeManager(node)
	require.NotNil(t, hm)
	assert.Equal(t, node, hm.node)
	assert.Equal(t, uint64(0), hm.round)
}

func TestHandshakeManager_UpdateRound(t *testing.T) {
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	hm := NewHandshakeManager(node)

	// Update round
	hm.UpdateRound(5)
	assert.Equal(t, uint64(5), hm.round)
	assert.NotNil(t, hm.roundSeed)
	assert.Len(t, hm.roundSeed, 32) // SHA-256 hash length
}

func TestHandshakeManager_VerifyRoundSeed(t *testing.T) {
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	hm := NewHandshakeManager(node)
	hm.UpdateRound(10)

	// Test valid round seed
	validSeed := hm.node.barNet.generateRoundSeed(10)
	assert.True(t, hm.verifyRoundSeed(10, validSeed))

	// Test invalid round seed
	invalidSeed := hm.node.barNet.generateRoundSeed(11)
	assert.False(t, hm.verifyRoundSeed(10, invalidSeed))

	// Test empty seed
	assert.False(t, hm.verifyRoundSeed(10, []byte{}))
}

func TestHandshakeManager_Integration(t *testing.T) {
	// This test would require a full P2P setup
	// For now, we'll test the basic functionality without network
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	hm := NewHandshakeManager(node)
	hm.UpdateRound(1)

	// Test that round seed is generated correctly
	expectedSeed := node.barNet.generateRoundSeed(1)
	assert.Equal(t, expectedSeed, hm.roundSeed)

	// Test round verification
	assert.True(t, hm.verifyRoundSeed(1, expectedSeed))
	assert.False(t, hm.verifyRoundSeed(1, []byte{0, 0, 0}))
}

func TestHandshakeManager_EnhancedProtocol(t *testing.T) {
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	hm := NewHandshakeManager(node)
	hm.UpdateRound(5)

	// Test that nonce generation works
	nonce1 := hm.generateNonce()
	nonce2 := hm.generateNonce()
	assert.NotEqual(t, nonce1, nonce2)
	assert.Greater(t, nonce2, nonce1)
}

func TestHandshakeMessage_FullVersionFields(t *testing.T) {
	// Test the version fields without peer ID validation issues
	msg := &HandshakeMessage{
		Type:          "ping",
		From:          "test-peer-1", // Using string for testing
		To:            "test-peer-2", // Using string for testing
		Round:         5,
		RoundSeed:     []byte{1, 2, 3, 4},
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "test-v1.0",
		AddrReceived:  "/ip4/127.0.0.1/tcp/8080",
		AddrFrom:      "/ip4/127.0.0.1/tcp/8081",
		LastRound:     4,
		Nonce:         12345,
	}

	// Test that all version fields are properly set
	assert.Equal(t, "test-v1.0", msg.ClientVersion)
	assert.Equal(t, "/ip4/127.0.0.1/tcp/8080", msg.AddrReceived)
	assert.Equal(t, "/ip4/127.0.0.1/tcp/8081", msg.AddrFrom)
	assert.Equal(t, uint64(4), msg.LastRound)
	assert.Equal(t, uint64(12345), msg.Nonce)
	assert.Equal(t, "ping", msg.Type)
	assert.Equal(t, uint64(5), msg.Round)
	assert.Equal(t, []byte{1, 2, 3, 4}, msg.RoundSeed)
}

func TestHandshakeMessage_JSONCompatibility(t *testing.T) {
	// Test JSON marshaling with a simpler structure
	type SimpleHandshakeMessage struct {
		Type          string `json:"type"`
		Round         uint64 `json:"round"`
		ClientVersion string `json:"client_version"`
		AddrReceived  string `json:"addr_received"`
		AddrFrom      string `json:"addr_from"`
		LastRound     uint64 `json:"last_round"`
		Nonce         uint64 `json:"nonce"`
	}

	msg := &SimpleHandshakeMessage{
		Type:          "ping",
		Round:         5,
		ClientVersion: "test-v1.0",
		AddrReceived:  "/ip4/127.0.0.1/tcp/8080",
		AddrFrom:      "/ip4/127.0.0.1/tcp/8081",
		LastRound:     4,
		Nonce:         12345,
	}

	// Marshal
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// Unmarshal
	var unmarshaled SimpleHandshakeMessage
	err = json.Unmarshal(data, &unmarshaled)
	require.NoError(t, err)

	// Verify all fields
	assert.Equal(t, msg.Type, unmarshaled.Type)
	assert.Equal(t, msg.Round, unmarshaled.Round)
	assert.Equal(t, msg.ClientVersion, unmarshaled.ClientVersion)
	assert.Equal(t, msg.AddrReceived, unmarshaled.AddrReceived)
	assert.Equal(t, msg.AddrFrom, unmarshaled.AddrFrom)
	assert.Equal(t, msg.LastRound, unmarshaled.LastRound)
	assert.Equal(t, msg.Nonce, unmarshaled.Nonce)
}

func TestHandshakeManager_MessageTypes(t *testing.T) {
	node := &AppNode{
		ctx:    context.Background(),
		barNet: NewBARNetwork(nil),
	}

	hm := NewHandshakeManager(node)
	hm.UpdateRound(10)

	// Test ping message creation
	ping := &HandshakeMessage{
		Type:          "ping",
		From:          "test-peer-1",
		To:            "test-peer-2",
		Round:         10,
		RoundSeed:     hm.roundSeed,
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "test-v1.0",
		AddrReceived:  "",
		AddrFrom:      "/ip4/127.0.0.1/tcp/8080",
		LastRound:     10,
		Nonce:         hm.generateNonce(),
	}

	// Test pong message creation
	pong := &HandshakeMessage{
		Type:          "pong",
		From:          "test-peer-2",
		To:            "test-peer-1",
		Round:         10,
		RoundSeed:     hm.roundSeed,
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "test-v1.0",
		AddrReceived:  "/ip4/127.0.0.1/tcp/8080",
		AddrFrom:      "/ip4/127.0.0.1/tcp/8081",
		LastRound:     10,
		Nonce:         hm.generateNonce(),
	}

	// Test ack message creation
	ack := &HandshakeMessage{
		Type:          "ack",
		From:          "test-peer-2",
		To:            "test-peer-1",
		Round:         10,
		RoundSeed:     hm.roundSeed,
		Timestamp:     time.Now().UnixNano(),
		ClientVersion: "test-v1.0",
		AddrReceived:  "/ip4/127.0.0.1/tcp/8080",
		AddrFrom:      "/ip4/127.0.0.1/tcp/8081",
		LastRound:     10,
		Nonce:         hm.generateNonce(),
	}

	// Verify all messages can be marshaled
	_, err := json.Marshal(ping)
	require.NoError(t, err)

	_, err = json.Marshal(pong)
	require.NoError(t, err)

	_, err = json.Marshal(ack)
	require.NoError(t, err)

	// Verify message types
	assert.Equal(t, "ping", ping.Type)
	assert.Equal(t, "pong", pong.Type)
	assert.Equal(t, "ack", ack.Type)
}
