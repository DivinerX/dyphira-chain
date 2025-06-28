package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APIResponse represents the standard API response format
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Code    int         `json:"code,omitempty"`
	Message string      `json:"message,omitempty"`
}

// APIServer represents the REST API server
type APIServer struct {
	node            *AppNode
	shutdownManager *GracefulShutdown
	port            int
}

// NewAPIServer creates a new API server instance
func NewAPIServer(node *AppNode, shutdownManager *GracefulShutdown, port int) *APIServer {
	return &APIServer{
		node:            node,
		shutdownManager: shutdownManager,
		port:            port,
	}
}

// Start starts the API server
func (api *APIServer) Start() error {
	mux := http.NewServeMux()

	// API v1 routes
	apiV1 := http.NewServeMux()

	// Health check endpoint (legacy, also available at /api/v1/health)
	mux.HandleFunc("/health", api.handleLegacyHealth)

	// API v1 endpoints
	apiV1.HandleFunc("/health", api.handleHealth)
	apiV1.HandleFunc("/status", api.handleStatus)
	apiV1.HandleFunc("/network", api.handleNetwork)
	apiV1.HandleFunc("/validators", api.handleValidators)
	apiV1.HandleFunc("/blocks", api.handleBlocks)
	apiV1.HandleFunc("/transactions/pool", api.handleTransactionPool)
	apiV1.HandleFunc("/transactions/history", api.handleTransactionHistory)
	apiV1.HandleFunc("/transactions", api.handleCreateTransaction)
	apiV1.HandleFunc("/peers", api.handlePeers)
	apiV1.HandleFunc("/metrics", api.handleMetrics)
	apiV1.HandleFunc("/batch-stats", api.handleBatchStats)
	apiV1.HandleFunc("/state/snapshot", api.handleStateSnapshot)
	apiV1.HandleFunc("/transactions/", api.handleTransactionByHash)

	// Dynamic handler for accounts and blocks
	apiV1.HandleFunc("/", api.handleDynamic)

	// Mount API v1 under /api/v1
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiV1))

	// Legacy endpoints (for backward compatibility)
	mux.HandleFunc("/metrics", api.handleLegacyMetrics)
	mux.HandleFunc("/shutdown/status", api.handleShutdownStatus)
	mux.HandleFunc("/shutdown", api.handleShutdown)
	mux.HandleFunc("/node/info", api.handleNodeInfo)

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", api.port),
		Handler: mux,
	}

	go func() {
		log.Printf("API server starting on port %d", api.port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("API server error: %v", err)
		}
	}()

	return nil
}

// handleHealth handles the health check endpoint
func (api *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	api.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"node_id":   api.node.address.ToHex(),
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
		},
	})
}

// handleStatus handles the node status endpoint
func (api *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	// Get validator info
	validator, err := api.node.vr.GetValidator(api.node.address)
	isValidator := err == nil && validator != nil
	participating := isValidator && validator.Participating
	stake := uint64(0)
	delegatedStake := uint64(0)
	if isValidator {
		stake = validator.Stake
		delegatedStake = validator.DelegatedStake
	}

	api.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"node_id":           api.node.address.ToHex(),
			"blockchain_height": api.node.bc.Height(),
			"is_validator":      isValidator,
			"participating":     participating,
			"stake":             stake,
			"delegated_stake":   delegatedStake,
			"connected_peers":   len(api.node.p2p.host.Network().Peers()),
			"transaction_pool":  api.node.txPool.Size(),
		},
	})
}

// handleNetwork handles the network information endpoint
func (api *APIServer) handleNetwork(w http.ResponseWriter, r *http.Request) {
	// Get real bandwidth statistics
	bandwidthIn, bandwidthOut := api.node.p2p.GetBandwidthStats()

	// Update network metrics with real data
	peerCount := len(api.node.p2p.host.Network().Peers())
	api.node.metrics.UpdateNetworkMetrics(peerCount, bandwidthIn, bandwidthOut)

	// Get network metrics
	networkMetrics := api.node.metrics.GetNetworkMetrics()

	// Add additional network info
	validators, _ := api.node.vr.GetAllValidators()
	activeValidators := 0
	for _, v := range validators {
		if v.Participating {
			activeValidators++
		}
	}

	// Calculate average block time
	consensusMetrics := api.node.metrics.GetConsensusMetrics()
	avgBlockTime := consensusMetrics["block_time"].(float64)

	// Calculate transaction TPS
	performanceMetrics := api.node.metrics.GetPerformanceMetrics()
	transactionTPS := performanceMetrics["transaction_tps"].(float64)

	api.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"connected_peers":    networkMetrics["connected_peers"],
			"total_validators":   len(validators),
			"active_validators":  activeValidators,
			"current_height":     api.node.bc.Height(),
			"transaction_pool":   api.node.txPool.Size(),
			"average_block_time": avgBlockTime,
			"transaction_tps":    transactionTPS,
			"bandwidth_in":       bandwidthIn,
			"bandwidth_out":      bandwidthOut,
		},
	})
}

// handleValidators handles the validators endpoint
func (api *APIServer) handleValidators(w http.ResponseWriter, r *http.Request) {
	validators, err := api.node.vr.GetAllValidators()
	if err != nil {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Failed to get validators",
			Code:    500,
		})
		return
	}

	validatorData := make([]map[string]interface{}, 0, len(validators))
	for _, v := range validators {
		validatorData = append(validatorData, map[string]interface{}{
			"address":            v.Address.ToHex(),
			"stake":              v.Stake,
			"delegated_stake":    v.DelegatedStake,
			"compute_reputation": v.ComputeReputation,
			"participating":      v.Participating,
		})
	}

	api.writeJSON(w, APIResponse{
		Success: true,
		Data:    validatorData,
	})
}

// handleBlocks handles the blocks endpoint
func (api *APIServer) handleBlocks(w http.ResponseWriter, r *http.Request) {
	// Parse limit parameter
	limit := 10
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
			if limit > 100 {
				limit = 100
			}
		}
	}

	currentHeight := api.node.bc.Height()
	blocks := make([]map[string]interface{}, 0)

	// Get recent blocks (up to limit)
	for i := int(currentHeight); i >= 0 && len(blocks) < limit; i-- {
		block, err := api.node.bc.GetBlockByHeight(uint64(i))
		if err != nil {
			continue
		}

		// Calculate block statistics
		totalValue := uint64(0)
		totalFees := uint64(0)
		txTypes := make(map[string]int)

		for _, tx := range block.Transactions {
			totalValue += tx.Value
			totalFees += tx.Fee
			txTypes[tx.Type]++
		}

		// Convert transactions to summary format
		txSummaries := make([]map[string]interface{}, 0, len(block.Transactions))
		for _, tx := range block.Transactions {
			txSummaries = append(txSummaries, map[string]interface{}{
				"hash":  tx.Hash.ToHex(),
				"from":  tx.From.ToHex(),
				"to":    tx.To.ToHex(),
				"value": tx.Value,
				"fee":   tx.Fee,
				"type":  tx.Type,
				"nonce": tx.Nonce,
			})
		}

		blocks = append(blocks, map[string]interface{}{
			"height":            block.Header.BlockNumber,
			"hash":              block.Header.Hash.ToHex(),
			"previous_hash":     block.Header.PreviousHash.ToHex(),
			"proposer":          block.Header.Proposer.ToHex(),
			"timestamp":         block.Header.Timestamp,
			"transaction_count": len(block.Transactions),
			"total_value":       totalValue,
			"total_fees":        totalFees,
			"transaction_types": txTypes,
			"transactions":      txSummaries,
		})
	}

	api.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"blocks":         blocks,
			"total_blocks":   len(blocks),
			"current_height": currentHeight,
		},
	})
}

// handleTransactionPool handles the transaction pool endpoint
func (api *APIServer) handleTransactionPool(w http.ResponseWriter, r *http.Request) {
	poolSize := api.node.txPool.Size()

	// Get transaction pool details
	transactions := api.node.txPool.GetTransactions()
	txData := make([]map[string]interface{}, 0, len(transactions))

	for _, tx := range transactions {
		txData = append(txData, map[string]interface{}{
			"hash":      tx.Hash.ToHex(),
			"from":      tx.From.ToHex(),
			"to":        tx.To.ToHex(),
			"value":     tx.Value,
			"fee":       tx.Fee,
			"nonce":     tx.Nonce,
			"type":      tx.Type,
			"timestamp": tx.Timestamp,
		})
	}

	api.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"pool_size":    poolSize,
			"transactions": txData,
			"total_value":  calculateTotalValue(transactions),
			"total_fees":   calculateTotalFees(transactions),
			"node_address": api.node.address.ToHex(),
		},
	})
}

// handleTransactionHistory handles the transaction history endpoint
func (api *APIServer) handleTransactionHistory(w http.ResponseWriter, r *http.Request) {
	// Parse query parameters
	address := r.URL.Query().Get("address")
	limitStr := r.URL.Query().Get("limit")
	offsetStr := r.URL.Query().Get("offset")

	limit := 50 // Default limit
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	offset := 0 // Default offset
	if offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Get transaction history from recent blocks
	var transactions []map[string]interface{}
	var targetAddr Address
	var err error

	if address != "" {
		targetAddr, err = HexToAddress(address)
		if err != nil {
			api.writeJSON(w, APIResponse{
				Success: false,
				Error:   "Invalid address format",
				Code:    400,
			})
			return
		}
	}

	// Search through recent blocks for transactions
	currentHeight := api.node.bc.Height()
	startHeight := uint64(0)
	if currentHeight > 100 {
		startHeight = currentHeight - 100 // Look at last 100 blocks
	}

	for h := currentHeight; h >= startHeight && len(transactions) < limit+offset; h-- {
		block, err := api.node.bc.GetBlockByHeight(h)
		if err != nil {
			continue
		}

		for _, tx := range block.Transactions {
			// Filter by address if specified
			if address != "" {
				if tx.From != targetAddr && tx.To != targetAddr {
					continue
				}
			}

			// Apply offset
			if offset > 0 {
				offset--
				continue
			}

			// Apply limit
			if len(transactions) >= limit {
				break
			}

			transactions = append(transactions, map[string]interface{}{
				"hash":      tx.Hash.ToHex(),
				"from":      tx.From.ToHex(),
				"to":        tx.To.ToHex(),
				"value":     tx.Value,
				"fee":       tx.Fee,
				"nonce":     tx.Nonce,
				"type":      tx.Type,
				"timestamp": tx.Timestamp,
				"block":     h,
				"confirmed": true,
			})
		}
	}

	api.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"transactions": transactions,
			"total":        len(transactions),
			"limit":        limit,
			"address":      address,
		},
	})
}

// Helper functions for transaction calculations
func calculateTotalValue(transactions []*Transaction) uint64 {
	total := uint64(0)
	for _, tx := range transactions {
		total += tx.Value
	}
	return total
}

func calculateTotalFees(transactions []*Transaction) uint64 {
	total := uint64(0)
	for _, tx := range transactions {
		total += tx.Fee
	}
	return total
}

// handleCreateTransaction handles the create transaction endpoint
func (api *APIServer) handleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Method not allowed",
			Code:    405,
		})
		return
	}

	// Parse request body
	var reqBody map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Invalid request body",
			Code:    400,
		})
		return
	}

	// Extract transaction parameters
	toStr, _ := reqBody["to"].(string)
	value, _ := reqBody["value"].(float64)
	fee, _ := reqBody["fee"].(float64)
	txType, _ := reqBody["type"].(string)

	// Validate required parameters
	if toStr == "" {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Missing 'to' address",
			Code:    400,
		})
		return
	}

	if value <= 0 {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Value must be greater than 0",
			Code:    400,
		})
		return
	}

	// Set default fee if not provided
	if fee <= 0 {
		fee = 1
	}

	// Parse recipient address
	toAddr, err := HexToAddress(toStr)
	if err != nil {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Invalid recipient address: " + err.Error(),
			Code:    400,
		})
		return
	}

	// Validate recipient is not the same as sender
	if toAddr == api.node.address {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Cannot send to yourself",
			Code:    400,
		})
		return
	}

	// Get sender account and validate balance
	senderAccount, err := api.node.state.GetAccount(api.node.address)
	if err != nil {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Sender account not found",
			Code:    404,
		})
		return
	}

	totalRequired := uint64(value) + uint64(fee)
	if senderAccount.Balance < totalRequired {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error: fmt.Sprintf("Insufficient balance. Required: %d (amount: %d + fee: %d), Available: %d",
				totalRequired, uint64(value), uint64(fee), senderAccount.Balance),
			Code: 400,
		})
		return
	}

	// Validate transaction type
	if txType == "" {
		txType = "transfer" // Default type
	}
	if txType != "transfer" && txType != "stake" && txType != "unstake" && txType != "delegate" && txType != "register_validator" {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Invalid transaction type. Must be one of: transfer, stake, unstake, delegate, register_validator",
			Code:    400,
		})
		return
	}

	// Create transaction
	tx := &Transaction{
		From:      api.node.address,
		To:        toAddr,
		Value:     uint64(value),
		Fee:       uint64(fee),
		Nonce:     senderAccount.Nonce + 1,
		Timestamp: time.Now().UnixNano(),
		Type:      txType,
	}

	// Sign transaction
	if err := tx.Sign(api.node.privKey); err != nil {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Failed to sign transaction: " + err.Error(),
			Code:    500,
		})
		return
	}

	// Add to transaction pool
	if err := api.node.txPool.AddTransaction(tx, api.node.privKey.PubKey(), api.node.state); err != nil {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   fmt.Sprintf("Failed to add transaction to pool: %v", err),
			Code:    500,
		})
		return
	}

	// Broadcast transaction
	txData, _ := tx.Encode()
	if err := api.node.p2p.Publish(api.node.ctx, TransactionTopic, txData); err != nil {
		log.Printf("Failed to broadcast transaction: %v", err)
		// Don't fail the request if broadcasting fails
	}

	// Update metrics
	api.node.metrics.RecordTransaction()

	// Return detailed transaction information
	api.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"transaction": map[string]interface{}{
				"hash":      tx.Hash.ToHex(),
				"from":      tx.From.ToHex(),
				"to":        tx.To.ToHex(),
				"value":     tx.Value,
				"fee":       tx.Fee,
				"nonce":     tx.Nonce,
				"type":      tx.Type,
				"timestamp": tx.Timestamp,
			},
			"summary": map[string]interface{}{
				"total_cost":     totalRequired,
				"balance_before": senderAccount.Balance,
				"balance_after":  senderAccount.Balance - totalRequired,
				"pool_size":      api.node.txPool.Size(),
			},
			"status": "broadcasted",
		},
		Message: "Transaction created and broadcast successfully",
	})
}

// handlePeers handles the peers endpoint
func (api *APIServer) handlePeers(w http.ResponseWriter, r *http.Request) {
	peers := api.node.p2p.host.Network().Peers()
	peerData := make([]map[string]interface{}, 0, len(peers))

	for _, peer := range peers {
		// Get peer connection info
		conns := api.node.p2p.host.Network().ConnsToPeer(peer)
		peerInfo := map[string]interface{}{
			"id": peer.String(),
		}

		if len(conns) > 0 {
			// Get the first connection's remote address
			remoteAddr := conns[0].RemoteMultiaddr().String()
			peerInfo["address"] = remoteAddr
			peerInfo["connected"] = true
		} else {
			peerInfo["connected"] = false
		}

		peerData = append(peerData, peerInfo)
	}

	api.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"peers":       peerData,
			"total_peers": len(peers),
			"node_id":     api.node.address.ToHex(),
			"listen_addr": api.node.p2p.GetListenAddr(),
			"listen_port": api.node.p2p.GetListenPort(),
		},
	})
}

// handleMetrics handles the detailed metrics endpoint
func (api *APIServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// Update performance metrics
	api.node.metrics.UpdatePerformanceMetrics()

	// Get real bandwidth statistics
	bandwidthIn, bandwidthOut := api.node.p2p.GetBandwidthStats()

	// Update network metrics with real data
	peerCount := len(api.node.p2p.host.Network().Peers())
	api.node.metrics.UpdateNetworkMetrics(peerCount, bandwidthIn, bandwidthOut)

	// Update consensus metrics
	validators, _ := api.node.vr.GetAllValidators()
	activeValidators := 0
	for _, v := range validators {
		if v.Participating {
			activeValidators++
		}
	}

	// Calculate approval rate (simplified)
	approvalRate := 0.0
	if len(api.node.committee) > 0 {
		approvalRate = float64(activeValidators) / float64(len(api.node.committee))
	}

	api.node.metrics.UpdateConsensusMetrics(
		api.node.bc.Height(),
		len(api.node.committee),
		approvalRate,
	)

	// Get real database size
	databaseSize := api.node.bc.GetDatabaseSize()

	// Update storage metrics with real data
	api.node.metrics.UpdateStorageMetrics(
		databaseSize,
		uint64(api.node.txPool.Size()),
		api.node.bc.Height()+1,
		len(validators),
	)

	// Get comprehensive metrics
	metrics := api.node.metrics.GetMetrics()

	api.writeJSON(w, APIResponse{
		Success: true,
		Data:    metrics,
	})
}

// handleDynamic handles dynamic routes for accounts and blocks
func (api *APIServer) handleDynamic(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	log.Printf("DEBUG: Dynamic handler called with path: %s", path)

	// Handle accounts
	if strings.HasPrefix(path, "/accounts/") {
		api.handleAccount(w, r)
		return
	}

	// Handle blocks
	if strings.HasPrefix(path, "/blocks/") {
		api.handleBlockByHeight(w, r)
		return
	}

	// If no pattern matches, return 404
	http.NotFound(w, r)
}

// handleAccount handles account-related requests
func (api *APIServer) handleAccount(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	addrStr := path[len("/accounts/"):]
	log.Printf("DEBUG: Account request, address string: '%s'", addrStr)

	// Check if it's a balance request
	if len(addrStr) > 8 && addrStr[len(addrStr)-8:] == "/balance" {
		addrStr = addrStr[:len(addrStr)-8]
		log.Printf("DEBUG: Balance request, trimmed address: '%s'", addrStr)
	}

	addr, err := HexToAddress(addrStr)
	if err != nil {
		log.Printf("DEBUG: HexToAddress failed: %v", err)
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Invalid address",
			Code:    400,
		})
		return
	}

	account, err := api.node.state.GetAccount(addr)
	if err != nil {
		api.writeJSON(w, APIResponse{
			Success: false,
			Error:   "Account not found",
			Code:    404,
		})
		return
	}

	// Check if it's a balance-only request
	if len(path) > 8 && path[len(path)-8:] == "/balance" {
		api.writeJSON(w, APIResponse{
			Success: true,
			Data: map[string]interface{}{
				"address": addr.ToHex(),
				"balance": account.Balance,
			},
		})
		return
	}

	// Full account information
	api.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"address": addr.ToHex(),
			"balance": account.Balance,
			"nonce":   account.Nonce,
		},
	})
}

// handleBlockByHeight handles GET /blocks/{height}
func (api *APIServer) handleBlockByHeight(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeJSON(w, APIResponse{Success: false, Error: "Method not allowed", Code: 405})
		return
	}
	path := r.URL.Path
	prefix := "/blocks/"
	if !strings.HasPrefix(path, prefix) {
		api.writeJSON(w, APIResponse{Success: false, Error: "Invalid path", Code: 400})
		return
	}
	heightStr := strings.TrimPrefix(path, prefix)
	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		api.writeJSON(w, APIResponse{Success: false, Error: "Invalid block height", Code: 400})
		return
	}
	block, err := api.node.bc.GetBlockByHeight(height)
	if err != nil || block == nil {
		api.writeJSON(w, APIResponse{Success: false, Error: "Block not found", Code: 404})
		return
	}
	api.writeJSON(w, APIResponse{Success: true, Data: block})
}

// handleTransactionByHash handles GET /transactions/{hash}
func (api *APIServer) handleTransactionByHash(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		api.writeJSON(w, APIResponse{Success: false, Error: "Method not allowed", Code: 405})
		return
	}

	path := r.URL.Path
	prefixes := []string{"/api/v1/transactions/", "/transactions/"}
	var txHashStr string
	matched := false
	for _, prefix := range prefixes {
		if strings.HasPrefix(path, prefix) {
			txHashStr = strings.TrimPrefix(path, prefix)
			matched = true
			break
		}
	}
	if !matched {
		api.writeJSON(w, APIResponse{Success: false, Error: "Invalid path", Code: 400})
		return
	}

	if len(txHashStr) != 64 {
		api.writeJSON(w, APIResponse{Success: false, Error: "Invalid transaction hash length", Code: 400})
		return
	}

	// Parse transaction hash
	txHashBytes, err := hex.DecodeString(txHashStr)
	if err != nil {
		api.writeJSON(w, APIResponse{Success: false, Error: "Invalid transaction hash format", Code: 400})
		return
	}

	var txHash Hash
	copy(txHash[:], txHashBytes)

	// Find transaction in blockchain
	tx, blockHeight, err := api.node.bc.GetTransactionByHash(txHash)
	if err != nil {
		api.writeJSON(w, APIResponse{Success: false, Error: "Transaction not found", Code: 404})
		return
	}

	// Check if transaction is also in pool
	inPool := false
	poolTxs := api.node.txPool.GetTransactions()
	for _, poolTx := range poolTxs {
		if poolTx.Hash == txHash {
			inPool = true
			break
		}
	}

	api.writeJSON(w, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"hash":         tx.Hash.ToHex(),
			"from":         tx.From.ToHex(),
			"to":           tx.To.ToHex(),
			"value":        tx.Value,
			"fee":          tx.Fee,
			"nonce":        tx.Nonce,
			"type":         tx.Type,
			"timestamp":    tx.Timestamp,
			"block_height": blockHeight,
			"in_pool":      inPool,
			"confirmed":    true,
		},
	})
}

// Legacy handlers for backward compatibility

func (api *APIServer) handleLegacyHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func (api *APIServer) handleLegacyMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	metrics := map[string]interface{}{
		"block_height": api.node.bc.Height(),
		"peer_count":   len(api.node.p2p.host.Network().Peers()),
		"uptime":       time.Since(time.Now()).String(), // This would be calculated properly
	}
	json.NewEncoder(w).Encode(metrics)
}

func (api *APIServer) handleShutdownStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	status := api.shutdownManager.Status()
	json.NewEncoder(w).Encode(status)
}

func (api *APIServer) handleShutdown(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Shutdown initiated",
	})
	go func() {
		time.Sleep(100 * time.Millisecond) // Give response time to be sent
		api.shutdownManager.Shutdown("API request")
	}()
}

func (api *APIServer) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	info := map[string]interface{}{
		"address": api.node.address.ToHex(),
		"port":    api.node.p2p.GetListenPort(),
		"peers":   len(api.node.p2p.host.Network().Peers()),
	}
	json.NewEncoder(w).Encode(info)
}

// handleBatchStats handles the transaction batch statistics endpoint
func (api *APIServer) handleBatchStats(w http.ResponseWriter, r *http.Request) {
	stats := api.node.txPool.GetBatchStatistics()

	api.writeJSON(w, APIResponse{
		Success: true,
		Data:    stats,
	})
}

// handleStateSnapshot handles GET (export) and POST (import) for state snapshot
func (api *APIServer) handleStateSnapshot(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		accounts, err := api.node.state.ExportSnapshot()
		if err != nil {
			api.writeJSON(w, APIResponse{Success: false, Error: "Failed to export snapshot", Code: 500})
			return
		}
		api.writeJSON(w, APIResponse{Success: true, Data: accounts})
		return
	}
	if r.Method == http.MethodPost {
		var accounts []*Account
		if err := json.NewDecoder(r.Body).Decode(&accounts); err != nil {
			api.writeJSON(w, APIResponse{Success: false, Error: "Invalid snapshot data", Code: 400})
			return
		}
		if err := api.node.state.ImportSnapshot(accounts); err != nil {
			api.writeJSON(w, APIResponse{Success: false, Error: "Failed to import snapshot", Code: 500})
			return
		}
		api.writeJSON(w, APIResponse{Success: true, Message: "Snapshot imported successfully"})
		return
	}
	w.WriteHeader(http.StatusMethodNotAllowed)
}

// writeJSON writes a JSON response with proper headers
func (api *APIServer) writeJSON(w http.ResponseWriter, response APIResponse) {
	w.Header().Set("Content-Type", "application/json")

	// Set status code if provided
	if response.Code != 0 {
		w.WriteHeader(response.Code)
	}

	json.NewEncoder(w).Encode(response)
}
