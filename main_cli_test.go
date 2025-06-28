package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestCLIHelpAndExit(t *testing.T) {
	// Create a mock node for testing
	node := &AppNode{
		address: Address{},
		state:   &State{},
		vr:      &ValidatorRegistry{},
		bc:      &Blockchain{},
		p2p:     &P2PNode{},
	}

	// Create input with help and exit commands
	input := strings.NewReader("help\nexit\n")
	var output bytes.Buffer

	// Create CLI instance
	cli := NewCLI(node, input, &output)

	// Start CLI (this will process the input and exit)
	cli.Start()

	// Check output contains expected content
	outputStr := output.String()
	if !strings.Contains(outputStr, "Dyphira L1 DPoS Blockchain CLI") {
		t.Errorf("Expected CLI banner, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Available commands:") {
		t.Errorf("Expected help output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Goodbye!") {
		t.Errorf("Expected exit message, got: %s", outputStr)
	}
}

func TestCLIBalanceCommand(t *testing.T) {
	// Create a mock node with a test account
	testAddr := Address{}
	copy(testAddr[:], []byte("test_address_123456"))

	// Create state with test account
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
	}

	// Create input with balance command
	input := strings.NewReader("balance\nexit\n")
	var output bytes.Buffer

	// Create CLI instance
	cli := NewCLI(node, input, &output)

	// Start CLI
	cli.Start()

	// Check output contains expected content
	outputStr := output.String()
	if !strings.Contains(outputStr, "Balance: 1000") {
		t.Errorf("Expected balance output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Nonce: 5") {
		t.Errorf("Expected nonce output, got: %s", outputStr)
	}
}

func TestCLIAccountCommand(t *testing.T) {
	// Create a mock node with a test account
	testAddr := Address{}
	copy(testAddr[:], []byte("test_address_123456"))

	// Create state with test account
	state := NewState()
	testAccount := &Account{
		Address: testAddr,
		Balance: 1000,
		Nonce:   5,
	}
	state.PutAccount(testAccount)

	// Create a mock store for validator registry
	mockStore := &MockStorage{
		data: make(map[string][]byte),
	}
	validatorRegistry := NewValidatorRegistry(mockStore, "validators")

	node := &AppNode{
		address: testAddr,
		state:   state,
		vr:      validatorRegistry,
		bc:      &Blockchain{},
		p2p:     &P2PNode{},
	}

	// Create input with account command
	input := strings.NewReader("account\nexit\n")
	var output bytes.Buffer

	// Create CLI instance
	cli := NewCLI(node, input, &output)

	// Start CLI
	cli.Start()

	// Check output contains expected content
	outputStr := output.String()
	if !strings.Contains(outputStr, "Account Details:") {
		t.Errorf("Expected account details header, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Balance: 1000") {
		t.Errorf("Expected balance output, got: %s", outputStr)
	}
	if !strings.Contains(outputStr, "Nonce: 5") {
		t.Errorf("Expected nonce output, got: %s", outputStr)
	}
}

// MockStorage implements the Storage interface for testing
type MockStorage struct {
	data map[string][]byte
}

func (m *MockStorage) Put(key []byte, value []byte) error {
	m.data[string(key)] = value
	return nil
}

func (m *MockStorage) Get(key []byte) ([]byte, error) {
	if value, exists := m.data[string(key)]; exists {
		return value, nil
	}
	return nil, nil
}

func (m *MockStorage) Delete(key []byte) error {
	delete(m.data, string(key))
	return nil
}

func (m *MockStorage) List() (map[string][]byte, error) {
	return m.data, nil
}

func (m *MockStorage) Close() error {
	return nil
}
