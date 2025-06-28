package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/stretchr/testify/assert"
)

// createTestAPIServer creates a test API server for testing
func createTestAPIServer(t *testing.T) (*APIServer, *AppNode, *GracefulShutdown) {
	ctx := context.Background()
	privKey, err := btcec.NewPrivateKey()
	assert.NoError(t, err)

	p2pPrivKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	assert.NoError(t, err)

	node, err := NewAppNode(ctx, 8080, "test.db", p2pPrivKey, privKey)
	assert.NoError(t, err)

	shutdownManager := NewGracefulShutdown()
	apiServer := NewAPIServer(node, shutdownManager, 8081)

	return apiServer, node, shutdownManager
}

// createTestHandler creates a test HTTP handler from the API server
func createTestHandler(apiServer *APIServer) http.Handler {
	mux := http.NewServeMux()

	// API v1 routes
	apiV1 := http.NewServeMux()

	// Health check endpoint (legacy, also available at /api/v1/health)
	mux.HandleFunc("/health", apiServer.handleLegacyHealth)

	// API v1 endpoints
	apiV1.HandleFunc("/health", apiServer.handleHealth)
	apiV1.HandleFunc("/status", apiServer.handleStatus)
	apiV1.HandleFunc("/network", apiServer.handleNetwork)
	apiV1.HandleFunc("/validators", apiServer.handleValidators)
	apiV1.HandleFunc("/blocks", apiServer.handleBlocks)
	apiV1.HandleFunc("/transactions/pool", apiServer.handleTransactionPool)
	apiV1.HandleFunc("/transactions", apiServer.handleCreateTransaction)
	apiV1.HandleFunc("/peers", apiServer.handlePeers)
	apiV1.HandleFunc("/metrics", apiServer.handleMetrics)
	apiV1.HandleFunc("/blocks/", apiServer.handleBlockByHeight)
	apiV1.HandleFunc("/transactions/", apiServer.handleTransactionByHash)

	// Dynamic handler for accounts, blocks, and transactions to avoid routing conflicts
	apiV1.HandleFunc("/", apiServer.handleDynamic)

	// Mount API v1 under /api/v1
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiV1))

	// Legacy endpoints (for backward compatibility)
	mux.HandleFunc("/metrics", apiServer.handleLegacyMetrics)
	mux.HandleFunc("/shutdown/status", apiServer.handleShutdownStatus)
	mux.HandleFunc("/shutdown", apiServer.handleShutdown)
	mux.HandleFunc("/node/info", apiServer.handleNodeInfo)

	return mux
}

func TestAPIServer_HealthEndpoint(t *testing.T) {
	apiServer, node, _ := createTestAPIServer(t)
	defer func() {
		// Clean up resources
		if node.p2p != nil && node.p2p.host != nil {
			node.p2p.host.Close()
		}
		if node.chainStore != nil {
			node.chainStore.Close()
		}
		if node.validatorStore != nil {
			node.validatorStore.Close()
		}
	}()

	handler := createTestHandler(apiServer)

	// Test health endpoint
	req, _ := http.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "healthy", response["status"])
	assert.NotEmpty(t, response["time"])
}

func TestAPIServer_MetricsEndpoint(t *testing.T) {
	apiServer, node, _ := createTestAPIServer(t)
	defer func() {
		// Clean up resources
		if node.p2p != nil && node.p2p.host != nil {
			node.p2p.host.Close()
		}
		if node.chainStore != nil {
			node.chainStore.Close()
		}
		if node.validatorStore != nil {
			node.validatorStore.Close()
		}
	}()

	handler := createTestHandler(apiServer)

	// Test metrics endpoint
	req, _ := http.NewRequest("GET", "/metrics", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Check that expected metrics are present
	assert.Contains(t, response, "block_height")
	assert.Contains(t, response, "peer_count")
	assert.Contains(t, response, "uptime")

	// Check that block_height is a number
	blockHeight, ok := response["block_height"].(float64)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, blockHeight, float64(0))
}

func TestAPIServer_ShutdownStatusEndpoint(t *testing.T) {
	apiServer, node, _ := createTestAPIServer(t)
	defer func() {
		// Clean up resources
		if node.p2p != nil && node.p2p.host != nil {
			node.p2p.host.Close()
		}
		if node.chainStore != nil {
			node.chainStore.Close()
		}
		if node.validatorStore != nil {
			node.validatorStore.Close()
		}
	}()

	handler := createTestHandler(apiServer)

	// Test shutdown status endpoint
	req, _ := http.NewRequest("GET", "/shutdown/status", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Check that the response contains expected fields
	assert.Contains(t, response, "Components")

	// Initially should have empty reason
	reason, exists := response["Reason"]
	if exists {
		assert.Empty(t, reason)
	}
}

func TestAPIServer_NodeInfoEndpoint(t *testing.T) {
	apiServer, node, _ := createTestAPIServer(t)
	defer func() {
		// Clean up resources
		if node.p2p != nil && node.p2p.host != nil {
			node.p2p.host.Close()
		}
		if node.chainStore != nil {
			node.chainStore.Close()
		}
		if node.validatorStore != nil {
			node.validatorStore.Close()
		}
	}()

	handler := createTestHandler(apiServer)

	// Test node info endpoint
	req, _ := http.NewRequest("GET", "/node/info", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)

	// Check that expected fields are present
	assert.Contains(t, response, "address")
	assert.Contains(t, response, "port")
	assert.Contains(t, response, "peers")

	// Check that address is a string and not empty
	address, ok := response["address"].(string)
	assert.True(t, ok)
	assert.NotEmpty(t, address)

	// Check that peers is a number
	peers, ok := response["peers"].(float64)
	assert.True(t, ok)
	assert.GreaterOrEqual(t, peers, float64(0))
}

func TestAPIServer_ShutdownEndpoint(t *testing.T) {
	apiServer, node, shutdownManager := createTestAPIServer(t)
	defer func() {
		// Clean up resources
		if node.p2p != nil && node.p2p.host != nil {
			node.p2p.host.Close()
		}
		if node.chainStore != nil {
			node.chainStore.Close()
		}
		if node.validatorStore != nil {
			node.validatorStore.Close()
		}
	}()

	handler := createTestHandler(apiServer)

	// Test shutdown endpoint with GET (should fail)
	req, _ := http.NewRequest("GET", "/shutdown", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusMethodNotAllowed, w.Code)

	// Test shutdown endpoint with POST (should succeed)
	req, _ = http.NewRequest("POST", "/shutdown", bytes.NewBuffer([]byte("{}")))
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]string
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, "Shutdown initiated", response["message"])

	// Wait a bit for the shutdown to be processed
	time.Sleep(200 * time.Millisecond)

	// Check that shutdown status now has a reason
	status := shutdownManager.Status()
	assert.NotEmpty(t, status.Reason)
}

func TestAPIServer_Integration(t *testing.T) {
	apiServer, node, _ := createTestAPIServer(t)
	defer func() {
		// Clean up resources
		if node.p2p != nil && node.p2p.host != nil {
			node.p2p.host.Close()
		}
		if node.chainStore != nil {
			node.chainStore.Close()
		}
		if node.validatorStore != nil {
			node.validatorStore.Close()
		}
	}()

	handler := createTestHandler(apiServer)

	// Test all endpoints in sequence
	endpoints := []string{"/health", "/metrics", "/node/info", "/shutdown/status"}

	for _, endpoint := range endpoints {
		t.Run(fmt.Sprintf("Endpoint_%s", endpoint), func(t *testing.T) {
			req, _ := http.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))
			assert.NotEmpty(t, w.Body.String())
		})
	}
}

func TestAPIServer_APIv1Endpoints(t *testing.T) {
	apiServer, node, _ := createTestAPIServer(t)
	defer func() {
		// Clean up resources
		if node.p2p != nil && node.p2p.host != nil {
			node.p2p.host.Close()
		}
		if node.chainStore != nil {
			node.chainStore.Close()
		}
		if node.validatorStore != nil {
			node.validatorStore.Close()
		}
	}()

	handler := createTestHandler(apiServer)

	// Test API v1 endpoints
	v1Endpoints := []string{
		"/api/v1/health",
		"/api/v1/status",
		"/api/v1/network",
		"/api/v1/validators",
		"/api/v1/blocks",
		"/api/v1/transactions/pool",
		"/api/v1/peers",
		"/api/v1/metrics",
	}

	for _, endpoint := range v1Endpoints {
		t.Run(fmt.Sprintf("APIv1_%s", endpoint), func(t *testing.T) {
			req, _ := http.NewRequest("GET", endpoint, nil)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)
			assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

			var response APIResponse
			err := json.Unmarshal(w.Body.Bytes(), &response)
			assert.NoError(t, err)
			assert.True(t, response.Success)
		})
	}
}

func TestAPIServer_AccountEndpoints(t *testing.T) {
	apiServer, node, _ := createTestAPIServer(t)
	defer func() {
		// Clean up resources
		if node.p2p != nil && node.p2p.host != nil {
			node.p2p.host.Close()
		}
		if node.chainStore != nil {
			node.chainStore.Close()
		}
		if node.validatorStore != nil {
			node.validatorStore.Close()
		}
	}()

	handler := createTestHandler(apiServer)

	// Test account endpoints with a valid address
	validAddress := node.address.ToHex()

	// Test account info endpoint
	req, _ := http.NewRequest("GET", "/api/v1/accounts/"+validAddress, nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)

	// Test account balance endpoint
	req, _ = http.NewRequest("GET", "/api/v1/accounts/"+validAddress+"/balance", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
}

func TestAPIServer_BlockEndpoints(t *testing.T) {
	apiServer, node, _ := createTestAPIServer(t)
	defer func() {
		// Clean up resources
		if node.p2p != nil && node.p2p.host != nil {
			node.p2p.host.Close()
		}
		if node.chainStore != nil {
			node.chainStore.Close()
		}
		if node.validatorStore != nil {
			node.validatorStore.Close()
		}
	}()

	handler := createTestHandler(apiServer)

	// Test block endpoints
	// First test the blocks list endpoint
	req, _ := http.NewRequest("GET", "/api/v1/blocks", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)

	// Test specific block endpoint (block 0 should exist)
	req, _ = http.NewRequest("GET", "/api/v1/blocks/0", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.True(t, response.Success)
}

func TestAPIServer_TransactionEndpoints(t *testing.T) {
	apiServer, node, _ := createTestAPIServer(t)
	defer func() {
		// Clean up resources
		if node.p2p != nil && node.p2p.host != nil {
			node.p2p.host.Close()
		}
		if node.chainStore != nil {
			node.chainStore.Close()
		}
		if node.validatorStore != nil {
			node.validatorStore.Close()
		}
	}()

	handler := createTestHandler(apiServer)

	// Test transaction lookup endpoint with a non-existent transaction
	req, _ := http.NewRequest("GET", "/api/v1/transactions/0000000000000000000000000000000000000000000000000000000000000000", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response APIResponse
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "Transaction not found")

	// Test transaction lookup with invalid hash
	req, _ = http.NewRequest("GET", "/api/v1/transactions/invalid", nil)
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	err = json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.False(t, response.Success)
	assert.Contains(t, response.Error, "Invalid transaction hash")
}
