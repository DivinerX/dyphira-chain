package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
)

func TestAPICreateTransaction(t *testing.T) {
	// Create a mock node for testing
	privKey, _ := btcec.NewPrivateKey()

	node := &AppNode{
		address: Address{},
		state:   NewState(),
		vr:      &ValidatorRegistry{},
		bc:      &Blockchain{},
		p2p:     &P2PNode{},
		txPool:  NewTransactionPool(),
		privKey: privKey,
	}

	// Create API server
	api := NewAPIServer(node, nil, 8081)

	// Create test transaction request
	reqBody := map[string]interface{}{
		"to":    "0000000000000000000000000000000000000001",
		"value": 100,
		"fee":   1,
		"type":  "transfer",
	}

	reqJSON, _ := json.Marshal(reqBody)
	req := httptest.NewRequest("POST", "/api/v1/transactions", bytes.NewBuffer(reqJSON))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	// Call the handler
	api.handleCreateTransaction(w, req)

	// Check response - should fail due to insufficient balance
	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400 (insufficient balance), got %d", w.Code)
	}

	var response APIResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Errorf("Failed to parse response: %v", err)
	}

	if response.Success {
		t.Errorf("Expected success=false due to insufficient balance, got %v", response.Success)
	}
}

func TestCLISendCommand(t *testing.T) {
	// Create a mock node with test account
	testAddr := Address{}
	copy(testAddr[:], []byte("test_address_123456"))

	state := NewState()
	testAccount := &Account{
		Address: testAddr,
		Balance: 1000,
		Nonce:   5,
	}
	state.PutAccount(testAccount)

	node := &AppNode{
		address: testAddr,
		state:   state,
		vr:      &ValidatorRegistry{},
		bc:      &Blockchain{},
		p2p:     &P2PNode{},
		txPool:  NewTransactionPool(),
	}

	// Test send command with valid parameters
	cli := NewCLI(node, nil, nil)

	// Test insufficient balance
	err := cli.cmdSend([]string{"0000000000000000000000000000000000000001", "2000"})
	if err == nil {
		t.Errorf("Expected error for insufficient balance, got nil")
	}

	// Test invalid address
	err = cli.cmdSend([]string{"invalid_address", "100"})
	if err == nil {
		t.Errorf("Expected error for invalid address, got nil")
	}

	// Test invalid amount
	err = cli.cmdSend([]string{"0000000000000000000000000000000000000001", "0"})
	if err == nil {
		t.Errorf("Expected error for zero amount, got nil")
	}
}

func TestCLICreateCommand(t *testing.T) {
	// Create a mock node with test account
	testAddr := Address{}
	copy(testAddr[:], []byte("test_address_123456"))

	state := NewState()
	testAccount := &Account{
		Address: testAddr,
		Balance: 1000,
		Nonce:   5,
	}
	state.PutAccount(testAccount)

	node := &AppNode{
		address: testAddr,
		state:   state,
		vr:      &ValidatorRegistry{},
		bc:      &Blockchain{},
		p2p:     &P2PNode{},
		txPool:  NewTransactionPool(),
	}

	cli := NewCLI(node, nil, nil)

	// Test invalid transaction type
	err := cli.cmdCreate([]string{"invalid_type", "0000000000000000000000000000000000000001", "100"})
	if err == nil {
		t.Errorf("Expected error for invalid transaction type, got nil")
	}

	// Test missing parameters
	err = cli.cmdCreate([]string{"transfer", "100"})
	if err == nil {
		t.Errorf("Expected error for missing parameters, got nil")
	}

	// Test invalid amount
	err = cli.cmdCreate([]string{"transfer", "0000000000000000000000000000000000000001", "0"})
	if err == nil {
		t.Errorf("Expected error for zero amount, got nil")
	}
}
