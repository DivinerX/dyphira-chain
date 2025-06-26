package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"bytes"

	"github.com/btcsuite/btcd/btcec/v2"
	btcec_ecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
)

const (
	TransactionTopic   = "/dyphira/transactions/v1"
	BlockTopic         = "/dyphira/blocks/v1"
	ApprovalTopic      = "/dyphira/approvals/v1"
	BlockRequestTopic  = "/dyphira/block-requests/v1"
	BlockResponseTopic = "/dyphira/block-responses/v1"
	ValidatorTopic     = "/dyphira/validators/v1"
	EpochLength        = 270 // blocks - matches specification
	CommitteeSize      = 30
)

// AppNode represents the full blockchain application.
type AppNode struct {
	p2p     *P2PNode
	bc      *Blockchain
	state   *State
	txPool  *TransactionPool
	vr      *ValidatorRegistry
	ctx     context.Context
	privKey *btcec.PrivateKey
	address Address

	// BAR Resilient Network
	barNet                *BARNetwork
	handshakeManager      *HandshakeManager
	optimisticPushManager *OptimisticPushManager
	inactivityMonitor     *InactivityMonitor

	// DPoS / Consensus
	committeeSelector *CommitteeSelector
	proposerSelector  *ProposerSelector
	committee         []*Validator
	pendingBlocks     map[Hash]*BlockApproval
	pendingBlocksMu   sync.RWMutex

	// Buffer for approvals received before the block
	approvalBuffer   map[Hash][]*Approval
	approvalBufferMu sync.Mutex

	// Inactivity tracking for committee members
	inactivity map[Address]int

	// Database stores
	chainStore     Storage
	validatorStore Storage

	// --- TESTING ONLY ---
	DisableTestTransactions bool // If true, disables addTestTransactions in Start()
}

// --- Add a global test hook for committee sync (for tests only) ---
var TestSyncCommittee func(newCommittee []*Validator)

// NewAppNode creates and initializes a new full blockchain node.
func NewAppNode(ctx context.Context, listenPort int, dbPath string, p2pPrivKey crypto.PrivKey, privKey *btcec.PrivateKey) (*AppNode, error) {
	// --- Storage ---
	chainStore, err := NewBoltStore(dbPath, "chain")
	if err != nil {
		return nil, fmt.Errorf("failed to open chain store: %w", err)
	}
	validatorDbPath := dbPath + ".validators"
	validatorStore, err := NewBoltStore(validatorDbPath, "validators")
	if err != nil {
		chainStore.Close() // Clean up chain store if validator store fails
		return nil, fmt.Errorf("failed to open validator store: %w", err)
	}

	// --- Blockchain Components ---
	bc, err := NewBlockchain(chainStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockchain: %w", err)
	}
	state := NewState()
	vr := NewValidatorRegistry(validatorStore, "validators")
	txPool := NewTransactionPool()

	// Clear any existing validators to ensure clean state
	if err := vr.ClearAllValidators(); err != nil {
		return nil, fmt.Errorf("failed to clear validator registry: %w", err)
	}

	// --- P2P Component ---
	p2p, err := NewP2PNode(ctx, listenPort, p2pPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create P2P node: %w", err)
	}

	// --- BAR Network ---
	barNet := NewBARNetwork(nil) // Use default config for now

	// Wire up BAR integration: add peer to BAR network on connect
	p2p.OnPeerConnect = func(peerID peer.ID, address string) {
		barNet.AddPeer(peerID, address)
	}

	// Use the ECDSA public key to derive the blockchain address
	addr := pubKeyToAddress(privKey.PubKey())

	node := &AppNode{
		p2p:               p2p,
		bc:                bc,
		state:             state,
		txPool:            txPool,
		vr:                vr,
		ctx:               ctx,
		privKey:           privKey,
		address:           addr,
		barNet:            barNet,
		handshakeManager:  nil, // Will be initialized after node creation
		committeeSelector: &CommitteeSelector{Registry: vr},
		pendingBlocks:     make(map[Hash]*BlockApproval),
		approvalBuffer:    make(map[Hash][]*Approval),
		inactivity:        make(map[Address]int),
		chainStore:        chainStore,
		validatorStore:    validatorStore,
	}

	// --- Handshake Manager ---
	node.handshakeManager = NewHandshakeManager(node)

	// --- Optimistic Push Manager ---
	node.optimisticPushManager = NewOptimisticPushManager(node)

	// --- Inactivity Monitor ---
	node.inactivityMonitor = NewInactivityMonitor(node, false) // Default to non-seed node

	// Wire up optimistic push for newly promoted peers
	barNet.SetOnPeerStatusChange(func(peerID peer.ID, oldStatus, newStatus PeerStatus) {
		if oldStatus == PeerStatusGreylist && newStatus == PeerStatusWhitelist {
			// Trigger optimistic push for newly promoted peer
			node.optimisticPushManager.TriggerOptimisticPushForNewPeer(peerID)
		}
	})

	// Register self as a validator (participating by default)
	if err := vr.RegisterValidator(&Validator{Address: addr, Stake: 100, Participating: true}); err != nil {
		return nil, fmt.Errorf("failed to register self as validator: %w", err)
	}

	// Give initial balance to the validator account
	initialAccount := &Account{
		Address: addr,
		Balance: 1000, // Initial balance for testing
		Nonce:   0,
	}
	if err := state.PutAccount(initialAccount); err != nil {
		return nil, fmt.Errorf("failed to set initial account balance: %w", err)
	}

	log.Printf("Node %s registered as validator (ECDSA address, participating) with initial balance %d", addr.ToHex(), initialAccount.Balance)

	return node, nil
}

// NewAppNodeWithStores creates a node with explicit chain and validator stores (for testing).
func NewAppNodeWithStores(ctx context.Context, listenPort int, p2pPrivKey crypto.PrivKey, privKey *btcec.PrivateKey, chainStore Storage, validatorStore Storage) (*AppNode, error) {
	// --- Blockchain Components ---
	bc, err := NewBlockchain(chainStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockchain: %w", err)
	}
	state := NewState()
	vr := NewValidatorRegistry(validatorStore, "validators")
	txPool := NewTransactionPool()

	// Clear any existing validators to ensure clean state
	if err := vr.ClearAllValidators(); err != nil {
		return nil, fmt.Errorf("failed to clear validator registry: %w", err)
	}

	// --- P2P Component ---
	p2p, err := NewP2PNode(ctx, listenPort, p2pPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create P2P node: %w", err)
	}

	// --- BAR Network ---
	barNet := NewBARNetwork(nil) // Use default config for now

	// Wire up BAR integration: add peer to BAR network on connect
	p2p.OnPeerConnect = func(peerID peer.ID, address string) {
		barNet.AddPeer(peerID, address)
	}

	// Use the ECDSA public key to derive the blockchain address
	addr := pubKeyToAddress(privKey.PubKey())

	node := &AppNode{
		p2p:               p2p,
		bc:                bc,
		state:             state,
		txPool:            txPool,
		vr:                vr,
		ctx:               ctx,
		privKey:           privKey,
		address:           addr,
		barNet:            barNet,
		handshakeManager:  nil, // Will be initialized after node creation
		committeeSelector: &CommitteeSelector{Registry: vr},
		pendingBlocks:     make(map[Hash]*BlockApproval),
		approvalBuffer:    make(map[Hash][]*Approval),
		inactivity:        make(map[Address]int),
		chainStore:        chainStore,
		validatorStore:    validatorStore,
	}

	// --- Handshake Manager ---
	node.handshakeManager = NewHandshakeManager(node)

	// --- Optimistic Push Manager ---
	node.optimisticPushManager = NewOptimisticPushManager(node)

	// --- Inactivity Monitor ---
	node.inactivityMonitor = NewInactivityMonitor(node, false) // Default to non-seed node

	// Wire up optimistic push for newly promoted peers
	barNet.SetOnPeerStatusChange(func(peerID peer.ID, oldStatus, newStatus PeerStatus) {
		if oldStatus == PeerStatusGreylist && newStatus == PeerStatusWhitelist {
			// Trigger optimistic push for newly promoted peer
			node.optimisticPushManager.TriggerOptimisticPushForNewPeer(peerID)
		}
	})

	// Register self as a validator (participating by default)
	if err := vr.RegisterValidator(&Validator{Address: addr, Stake: 100, Participating: true}); err != nil {
		return nil, fmt.Errorf("failed to register self as validator: %w", err)
	}

	// Give initial balance to the validator account
	initialAccount := &Account{
		Address: addr,
		Balance: 1000, // Initial balance for testing
		Nonce:   0,
	}
	if err := state.PutAccount(initialAccount); err != nil {
		return nil, fmt.Errorf("failed to set initial account balance: %w", err)
	}

	log.Printf("Node %s registered as validator (ECDSA address, participating) with initial balance %d", addr.ToHex(), initialAccount.Balance)
	return node, nil
}

// Close gracefully shuts down the node.
func (n *AppNode) Close() error {
	log.Printf("Closing node %s", n.address.ToHex())
	// Close the p2p host, blockchain db, etc.
	if err := n.p2p.host.Close(); err != nil {
		log.Printf("Error closing p2p host: %v", err)
		// Continue closing other resources
	}
	if err := n.chainStore.Close(); err != nil {
		log.Printf("Error closing chain store: %v", err)
	}
	if err := n.validatorStore.Close(); err != nil {
		log.Printf("Error closing validator store: %v", err)
	}
	return nil
}

// Start begins the node's operation.
func (n *AppNode) Start() error {
	n.p2p.RegisterTopic(TransactionTopic)
	n.p2p.RegisterTopic(BlockTopic)
	n.p2p.RegisterTopic(ApprovalTopic)
	n.p2p.RegisterTopic(BlockRequestTopic)
	n.p2p.RegisterTopic(BlockResponseTopic)
	n.p2p.RegisterTopic(ValidatorTopic)

	go n.p2p.Subscribe(n.ctx, n.handleNetworkMessage)
	go n.p2p.Discover(n.ctx)
	go n.producerLoop()

	// Start BAR handshake manager
	n.handshakeManager.Start(n.ctx)

	// Start BAR optimistic push manager
	n.optimisticPushManager.Start(n.ctx)

	// Start BAR inactivity monitor
	n.inactivityMonitor.Start(n.ctx)

	// Broadcast our validator registration to the network
	go n.broadcastValidatorRegistration()

	// After a short delay, initialize all known validator accounts with a starting balance, then add test transactions
	if !n.DisableTestTransactions {
		go func() {
			time.Sleep(8 * time.Second)
			validators, err := n.vr.GetAllValidators()
			if err != nil {
				log.Printf("ERROR: Failed to get all validators for initial account setup: %v", err)
				return
			}
			for _, v := range validators {
				acc, _ := n.state.GetAccount(v.Address)
				if acc.Balance == 0 {
					acc.Balance = 1000
					if err := n.state.PutAccount(acc); err != nil {
						log.Printf("ERROR: Failed to set initial balance for validator %s: %v", v.Address.ToHex(), err)
					}
					log.Printf("INFO: Initialized account for validator %s with balance 1000", v.Address.ToHex())
				}
			}
			n.addTestTransactions()
		}()
	}

	return nil
}

// --- Message Handling ---

func (n *AppNode) handleNetworkMessage(topic string, msg *pubsub.Message) {
	switch topic {
	case TransactionTopic:
		n.handleTransaction(msg)
	case BlockTopic:
		n.handleBlockProposal(msg)
	case ApprovalTopic:
		n.handleApproval(msg)
	case BlockRequestTopic:
		n.handleBlockRequest(msg)
	case BlockResponseTopic:
		n.handleBlockResponse(msg)
	case ValidatorTopic:
		n.handleValidatorRegistration(msg)
	}
}

func (n *AppNode) handleTransaction(msg *pubsub.Message) {
	log.Printf("DEBUG: Node %s received transaction message, data size: %d bytes", n.address.ToHex(), len(msg.Data))

	var netTx NetworkTransaction
	if err := json.Unmarshal(msg.Data, &netTx); err != nil {
		log.Printf("Failed to decode network tx: %v", err)
		// BAR: Update POM score for malformed message
		n.barNet.UpdatePOMScore(msg.ReceivedFrom, 1, "malformed transaction message")
		return
	}

	log.Printf("DEBUG: Node %s received transaction %s", n.address.ToHex(), netTx.Tx.Hash.ToHex())

	pubKey, err := UnmarshalPublicKey(netTx.PubKey)
	if err != nil {
		log.Printf("Failed to unmarshal public key: %v", err)
		// BAR: Update POM score for invalid public key
		n.barNet.UpdatePOMScore(msg.ReceivedFrom, 1, "invalid public key")
		return
	}

	if err := n.txPool.AddTransaction(netTx.Tx, pubKey, n.state); err != nil {
		log.Printf("Failed to add transaction to pool: %v", err)
		// BAR: Update POM score for invalid transaction
		n.barNet.UpdatePOMScore(msg.ReceivedFrom, 1, "invalid transaction")
		return
	}

	log.Printf("SUCCESS: Node %s added transaction %s to pool", n.address.ToHex(), netTx.Tx.Hash.ToHex())
}

func (n *AppNode) handleBlockProposal(msg *pubsub.Message) {
	log.Printf("DEBUG: Node %s received block message, data size: %d bytes", n.address.ToHex(), len(msg.Data))
	var block Block
	if err := json.Unmarshal(msg.Data, &block); err != nil {
		log.Printf("Failed to decode block proposal: %v", err)
		// BAR: Update POM score for malformed block
		n.barNet.UpdatePOMScore(msg.ReceivedFrom, 2, "malformed block message")
		return
	}
	n.processReceivedBlock(&block)
}

func (n *AppNode) processReceivedBlock(block *Block) {
	n.pendingBlocksMu.Lock()
	if _, exists := n.pendingBlocks[block.Header.Hash]; exists {
		log.Printf("DEBUG: Node %s already has block %s pending, ignoring.", n.address.ToHex(), block.Header.Hash.ToHex())
		n.pendingBlocksMu.Unlock()
		return
	}
	n.pendingBlocksMu.Unlock()

	if n.bc.HasBlock(block.Header.Hash) {
		log.Printf("DEBUG: Node %s already has block %s in the chain, ignoring.", n.address.ToHex(), block.Header.Hash.ToHex())
		return
	}

	currentHeight := n.bc.Height()
	maxAllowedHeight := currentHeight + 10 // Allow generous catch-up

	if block.Header.BlockNumber > maxAllowedHeight {
		log.Printf("WARN: Node %s ignoring block #%d too far ahead (max: %d), current height: %d",
			n.address.ToHex(), block.Header.BlockNumber, maxAllowedHeight, currentHeight)
		return
	}

	if block.Header.BlockNumber < currentHeight {
		log.Printf("WARN: Node %s ignoring old block #%d, current height: %d",
			n.address.ToHex(), block.Header.BlockNumber, currentHeight)
		return
	}

	log.Printf("DEBUG: Node %s processing block #%d, current height: %d", n.address.ToHex(), block.Header.BlockNumber, currentHeight)

	approval := NewBlockApproval(block, n.committee)
	n.pendingBlocksMu.Lock()
	n.pendingBlocks[block.Header.Hash] = approval
	n.pendingBlocksMu.Unlock()

	// Check the buffer for any approvals that arrived early
	n.approvalBufferMu.Lock()
	if bufferedApprovals, ok := n.approvalBuffer[block.Header.Hash]; ok {
		log.Printf("INFO: Node %s found %d buffered approvals for block %s", n.address.ToHex(), len(bufferedApprovals), block.Header.Hash.ToHex())
		for _, approvalMsg := range bufferedApprovals {
			// Intentionally not calling processApproval to avoid lock contention
			if err := approval.AddSignature(approvalMsg.Address, approvalMsg.Signature); err != nil {
				log.Printf("ERROR: Failed to add buffered approval signature for block %s: %v", approvalMsg.BlockHash.ToHex(), err)
			}
		}
		delete(n.approvalBuffer, block.Header.Hash)
	}
	n.approvalBufferMu.Unlock()

	go n.watchBlockApproval(block)

	isCommitteeMember := false
	for _, v := range n.committee {
		if v.Address == n.address {
			isCommitteeMember = true
			break
		}
	}

	if isCommitteeMember {
		log.Printf("INFO: Node %s (committee member) is voting for block #%d", n.address.ToHex(), block.Header.BlockNumber)
		sig := btcec_ecdsa.Sign(n.privKey, block.Header.Hash[:])
		if err := approval.AddSignature(n.address, sig.Serialize()); err != nil {
			log.Printf("ERROR: Failed to add self-approval: %v", err)
			n.pendingBlocksMu.Lock()
			delete(n.pendingBlocks, block.Header.Hash)
			n.pendingBlocksMu.Unlock()
			return
		}
		log.Printf("INFO: Node %s added self-approval for block %s. Total approvals: %d/%d",
			n.address.ToHex(), block.Header.Hash.ToHex(), len(approval.Signatures), approval.Threshold)

		// After self-approving, check if the block is ready for finalization.
		if approval.IsApproved() {
			go n.finalizeApprovedBlock(block)
		}

		n.broadcastApproval(block)
	} else {
		log.Printf("INFO: Node %s processing block #%d (not in committee, will not vote)",
			n.address.ToHex(), block.Header.BlockNumber)
	}
}

func (n *AppNode) handleApproval(msg *pubsub.Message) {
	var approvalMsg Approval
	if err := json.Unmarshal(msg.Data, &approvalMsg); err != nil {
		log.Printf("ERROR: Failed to decode approval: %v", err)
		// BAR: Update POM score for malformed approval
		n.barNet.UpdatePOMScore(msg.ReceivedFrom, 1, "malformed approval message")
		return
	}
	log.Printf("INFO: Node %s received approval for block %s from %s", n.address.ToHex(), approvalMsg.BlockHash.ToHex(), approvalMsg.Address.ToHex())
	n.processApproval(&approvalMsg)
}

func (n *AppNode) processApproval(approvalMsg *Approval) {
	n.pendingBlocksMu.RLock()
	approval, exists := n.pendingBlocks[approvalMsg.BlockHash]
	n.pendingBlocksMu.RUnlock()

	if !exists {
		n.approvalBufferMu.Lock()
		n.approvalBuffer[approvalMsg.BlockHash] = append(n.approvalBuffer[approvalMsg.BlockHash], approvalMsg)
		n.approvalBufferMu.Unlock()
		log.Printf("WARN: Node %s buffered approval for unknown block %s.", n.address.ToHex(), approvalMsg.BlockHash.ToHex())
		return
	}

	if err := approval.AddSignature(approvalMsg.Address, approvalMsg.Signature); err != nil {
		log.Printf("ERROR: Failed to add approval signature for block %s: %v", approvalMsg.BlockHash.ToHex(), err)
		return
	}
	log.Printf("INFO: Node %s added approval for block %s from %s. Total approvals: %d/%d",
		n.address.ToHex(), approvalMsg.BlockHash.ToHex(), approvalMsg.Address.ToHex(), len(approval.Signatures), approval.Threshold)

	// If the block is now approved, finalize it immediately.
	if approval.IsApproved() {
		go n.finalizeApprovedBlock(approval.Block)
	}
}

func (n *AppNode) watchBlockApproval(block *Block) {
	ticker := time.NewTicker(50 * time.Millisecond) // Poll frequently
	defer ticker.Stop()

	timeout := time.After(250 * time.Millisecond)

	for {
		select {
		case <-timeout:
			log.Printf("WARN: Timed out waiting for approvals for block %s", block.Header.Hash.ToHex())
			n.pendingBlocksMu.Lock()
			delete(n.pendingBlocks, block.Header.Hash)
			n.pendingBlocksMu.Unlock()
			return
		case <-ticker.C:
			n.pendingBlocksMu.RLock()
			approval, ok := n.pendingBlocks[block.Header.Hash]
			n.pendingBlocksMu.RUnlock()

			if !ok {
				log.Printf("DEBUG: watchBlockApproval exiting because pending block %s was removed", block.Header.Hash.ToHex())
				return
			}

			if approval.IsApproved() {
				go n.finalizeApprovedBlock(block)
				return // End the watch
			}
		}
	}
}

func (n *AppNode) BroadcastTransaction(tx *Transaction) error {
	pubKeyBytes := MarshalPublicKey(n.privKey.PubKey())
	netTx := &NetworkTransaction{
		Tx:     tx,
		PubKey: pubKeyBytes,
	}
	txBytes, err := json.Marshal(netTx)
	if err != nil {
		return fmt.Errorf("failed to encode transaction: %w", err)
	}
	log.Printf("Broadcasting transaction: %s", tx.Hash.ToHex())
	if err := n.p2p.Publish(n.ctx, TransactionTopic, txBytes); err != nil {
		return fmt.Errorf("failed to publish transaction: %w", err)
	}
	log.Printf("SUCCESS: Transaction %s published successfully", tx.Hash.ToHex())
	return nil
}

func (n *AppNode) broadcastApproval(block *Block) {
	sig := btcec_ecdsa.Sign(n.privKey, block.Header.Hash[:])
	approval := &Approval{
		BlockHash: block.Header.Hash,
		Address:   n.address,
		Signature: sig.Serialize(),
	}
	approvalBytes, err := json.Marshal(approval)
	if err != nil {
		log.Printf("ERROR: Failed to marshal approval: %v", err)
		return
	}
	if err := n.p2p.Publish(n.ctx, ApprovalTopic, approvalBytes); err != nil {
		log.Printf("ERROR: Failed to broadcast approval: %v", err)
	}
}

// producerLoop is the main loop for block production and consensus.
func (n *AppNode) producerLoop() {
	blockInterval := 2 * time.Second
	ticker := time.NewTicker(blockInterval)
	defer ticker.Stop()

	round := uint64(0)

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			round++

			// Update BAR network round
			n.handshakeManager.UpdateRound(round)

			// Use BAR network to select peers for this round
			selectedPeers := n.barNet.FindNodes(round)
			log.Printf("BAR: Round %d - Selected %d peers for connection", round, len(selectedPeers))

			// Log BAR network status
			whitelist := n.barNet.GetWhitelist()
			greylist := n.barNet.GetGreylist()
			banned := n.barNet.GetBanned()
			log.Printf("BAR: Status - Whitelist: %d, Greylist: %d, Banned: %d",
				len(whitelist), len(greylist), len(banned))

			// Cleanup old reputation records periodically
			if round%10 == 0 {
				n.barNet.CleanupReputationRecords()
			}

			// Continue with existing block production logic
			n.ForceCommitteeAndProposer()
		}
	}
}

func (n *AppNode) ForceCommitteeAndProposer() {
	currentHeight := n.bc.Height()
	epoch := currentHeight / EpochLength
	epochStartHeight := epoch * EpochLength

	// Check if we need to re-elect committee
	needReElection := false

	// Re-elect if no committee exists
	if n.committee == nil || len(n.committee) == 0 {
		needReElection = true
	}

	// Re-elect if we're at a new epoch
	if n.proposerSelector == nil || n.proposerSelector.EpochStart != epochStartHeight {
		needReElection = true
	}

	// Re-elect if we have fewer committee members than expected
	if len(n.committee) < CommitteeSize {
		needReElection = true
	}

	// Re-elect if we have new validators that aren't in the committee
	if n.committee != nil {
		committeeMap := make(map[Address]bool)
		for _, member := range n.committee {
			committeeMap[member.Address] = true
		}

		allValidators, err := n.vr.GetAllValidators()
		if err == nil {
			for _, v := range allValidators {
				if v.Participating && !committeeMap[v.Address] {
					needReElection = true
					break
				}
			}
		}
	}

	if !needReElection {
		return
	}

	newCommittee, err := n.committeeSelector.SelectCommittee(CommitteeSize)
	if err != nil {
		log.Printf("ERROR: Failed to get committee for epoch %d: %v", epoch, err)
		return
	}
	if len(newCommittee) == 0 {
		log.Printf("WARN: No validators found for epoch %d, cannot form committee.", epoch)
		return
	}

	// Log committee members for debugging
	committeeAddresses := make([]string, len(newCommittee))
	for i, member := range newCommittee {
		committeeAddresses[i] = member.Address.ToHex()
	}

	n.committee = newCommittee
	if TestSyncCommittee != nil {
		TestSyncCommittee(newCommittee)
	}

	n.proposerSelector = NewProposerSelectorWithRotation(newCommittee, epochStartHeight, EpochLength)
	log.Printf("INFO: Node %s elected new committee for epoch starting at height %d. Committee size: %d, members: %v",
		n.address.ToHex(), epochStartHeight, len(n.committee), committeeAddresses)
}

func ReplaceInactiveValidator(committee []*Validator, inactiveAddress Address, vr *ValidatorRegistry) ([]*Validator, error) {
	log.Printf("INFO: Replacing inactive validator %s", inactiveAddress.ToHex())

	// Get all validators from registry
	allValidators, err := vr.GetAllValidators()
	if err != nil {
		return nil, fmt.Errorf("failed to get all validators: %w", err)
	}

	// Create a map of current committee members for quick lookup
	committeeMap := make(map[Address]bool)
	for _, member := range committee {
		committeeMap[member.Address] = true
	}

	// Find participating validators not in the current committee
	var candidates []*Validator
	for _, v := range allValidators {
		if v.Participating && !committeeMap[v.Address] && v.Address != inactiveAddress {
			candidates = append(candidates, v)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no participating validators available to replace inactive validator %s", inactiveAddress.ToHex())
	}

	// Sort candidates by weight (stake + delegated stake + reputation) descending
	sort.Slice(candidates, func(i, j int) bool {
		weightI := candidates[i].Stake + candidates[i].DelegatedStake + candidates[i].ComputeReputation
		weightJ := candidates[j].Stake + candidates[j].DelegatedStake + candidates[j].ComputeReputation
		if weightI == weightJ {
			// For determinism, sort by address if weights are equal
			return bytes.Compare(candidates[i].Address[:], candidates[j].Address[:]) < 0
		}
		return weightI > weightJ
	})

	// Select the highest weighted candidate
	replacement := candidates[0]

	// Create new committee with the replacement
	newCommittee := make([]*Validator, len(committee))
	copy(newCommittee, committee)

	// Replace the inactive validator
	for i, member := range newCommittee {
		if member.Address == inactiveAddress {
			newCommittee[i] = replacement
			log.Printf("INFO: Replaced inactive validator %s with %s (weight: %d)",
				inactiveAddress.ToHex(), replacement.Address.ToHex(),
				replacement.Stake+replacement.DelegatedStake+replacement.ComputeReputation)
			break
		}
	}

	return newCommittee, nil
}

type BlockRequest struct {
	Height uint64  `json:"height"`
	From   Address `json:"from"`
}

type BlockResponse struct {
	Block *Block  `json:"block"`
	From  Address `json:"from"`
}

func (n *AppNode) handleBlockRequest(msg *pubsub.Message) {
	var request BlockRequest
	if err := json.Unmarshal(msg.Data, &request); err != nil {
		log.Printf("ERROR: Failed to decode block request: %v", err)
		return
	}

	if request.From == n.address {
		return
	}

	log.Printf("INFO: Node %s received block request for height %d from %s",
		n.address.ToHex(), request.Height, request.From.ToHex())

	block, err := n.bc.GetBlockByHeight(request.Height)
	if err != nil {
		log.Printf("DEBUG: Node %s doesn't have block #%d", n.address.ToHex(), request.Height)
		return
	}

	response := &BlockResponse{
		Block: block,
		From:  n.address,
	}
	responseBytes, err := json.Marshal(response)
	if err != nil {
		log.Printf("ERROR: Failed to marshal block response: %v", err)
		return
	}

	log.Printf("INFO: Node %s sending block #%d to %s", n.address.ToHex(), request.Height, request.From.ToHex())
	if err := n.p2p.Publish(n.ctx, BlockResponseTopic, responseBytes); err != nil {
		log.Printf("ERROR: Failed to publish block response: %v", err)
	}
}

func (n *AppNode) handleBlockResponse(msg *pubsub.Message) {
	var response BlockResponse
	if err := json.Unmarshal(msg.Data, &response); err != nil {
		log.Printf("ERROR: Failed to decode block response: %v", err)
		return
	}

	if response.From == n.address {
		return
	}

	log.Printf("INFO: Node %s received block response for block #%d from %s",
		n.address.ToHex(), response.Block.Header.BlockNumber, response.From.ToHex())

	n.processReceivedBlock(response.Block)
}

func (n *AppNode) RequestBlock(height uint64) {
	request := &BlockRequest{
		Height: height,
		From:   n.address,
	}

	requestBytes, err := json.Marshal(request)
	if err != nil {
		log.Printf("ERROR: Failed to marshal block request: %v", err)
		return
	}

	log.Printf("INFO: Node %s requesting block #%d", n.address.ToHex(), height)
	if err := n.p2p.Publish(n.ctx, BlockRequestTopic, requestBytes); err != nil {
		log.Printf("ERROR: Failed to publish block request: %v", err)
	}
}

func (n *AppNode) finalizeApprovedBlock(block *Block) {
	// Atomically check and remove the block from pending to "claim" it for finalization.
	n.pendingBlocksMu.Lock()
	_, ok := n.pendingBlocks[block.Header.Hash]
	if !ok {
		// Block was already finalized by another goroutine.
		n.pendingBlocksMu.Unlock()
		return
	}
	delete(n.pendingBlocks, block.Header.Hash)
	n.pendingBlocksMu.Unlock()

	log.Printf("SUCCESS: Node %s confirms block %s is now APPROVED!", n.address.ToHex(), block.Header.Hash.ToHex())
	if err := n.bc.AddBlock(block); err != nil {
		log.Printf("CRITICAL: Failed to add approved block %d to blockchain: %v", block.Header.BlockNumber, err)
		// If adding fails, we've already "claimed" it, so other nodes won't retry.
		// This is a critical state. For now, we just log and halt processing for this block.
		return
	}
	log.Printf("SUCCESS: Node %s added approved block %d to blockchain.", n.address.ToHex(), block.Header.BlockNumber)
	if err := n.bc.ApplyBlockWithRegistry(block, n.state, n.vr); err != nil {
		log.Printf("ERROR: Failed to apply block transactions to state: %v", err)
		return
	}
	log.Printf("SUCCESS: Node %s applied block %d transactions to state.", n.address.ToHex(), block.Header.BlockNumber)
	for _, tx := range block.Transactions {
		n.txPool.RemoveTransaction(tx.Hash)
	}
}

func (n *AppNode) broadcastValidatorRegistration() {
	// Wait a bit for connections to be established
	time.Sleep(3 * time.Second)

	// Send initial registration
	n.sendValidatorRegistration()

	// Send periodic registrations
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-n.ctx.Done():
			return
		case <-ticker.C:
			n.sendValidatorRegistration()
		}
	}
}

func (n *AppNode) sendValidatorRegistration() {
	// Get our validator info
	validator, err := n.vr.GetValidator(n.address)
	if err != nil || validator == nil {
		log.Printf("ERROR: Failed to get local validator info: %v", err)
		return
	}

	validatorRegistration := &ValidatorRegistration{
		Address: n.address,
		Stake:   validator.Stake,
	}
	registrationBytes, err := json.Marshal(validatorRegistration)
	if err != nil {
		log.Printf("ERROR: Failed to marshal validator registration: %v", err)
		return
	}
	if err := n.p2p.Publish(n.ctx, ValidatorTopic, registrationBytes); err != nil {
		log.Printf("ERROR: Failed to broadcast validator registration: %v", err)
	} else {
		log.Printf("DEBUG: Broadcast validator registration for %s with stake %d", n.address.ToHex(), validator.Stake)
	}
}

func (n *AppNode) handleValidatorRegistration(msg *pubsub.Message) {
	var registration ValidatorRegistration
	if err := json.Unmarshal(msg.Data, &registration); err != nil {
		log.Printf("ERROR: Failed to decode validator registration: %v", err)
		return
	}
	log.Printf("INFO: Node %s received validator registration for %s with stake %d", n.address.ToHex(), registration.Address.ToHex(), registration.Stake)

	// Register the validator in the registry
	if err := n.vr.RegisterValidator(&Validator{
		Address:       registration.Address,
		Stake:         registration.Stake,
		Participating: true,
	}); err != nil {
		log.Printf("ERROR: Failed to register validator: %v", err)
	} else {
		log.Printf("INFO: Node %s registered validator %s with stake %d", n.address.ToHex(), registration.Address.ToHex(), registration.Stake)
		// Force committee re-selection to include the new validator
		n.ForceCommitteeAndProposer()
		// Initialize the account for the new validator if it doesn't exist or has zero balance
		acc, _ := n.state.GetAccount(registration.Address)
		if acc.Balance == 0 {
			acc.Balance = 1000
			if err := n.state.PutAccount(acc); err != nil {
				log.Printf("ERROR: Failed to set initial balance for validator %s: %v", registration.Address.ToHex(), err)
			}
			log.Printf("INFO: Initialized account for validator %s with balance 1000 (on registration)", registration.Address.ToHex())
		}
	}
}

func (n *AppNode) addTestTransactions() {
	// Wait for the network to be ready
	time.Sleep(5 * time.Second)

	// Create a test recipient address (different from our own)
	testRecipient := Address{}
	copy(testRecipient[:], []byte("test_recipient_address"))

	// Create different types of test transactions
	transactions := []struct {
		to     Address
		value  uint64
		nonce  uint64
		txType string
	}{
		{testRecipient, 10, 1, "transfer"},
		{testRecipient, 25, 2, "transfer"},
		{testRecipient, 50, 3, "transfer"},
		{testRecipient, 100, 4, "transfer"},
		{testRecipient, 200, 5, "transfer"},
	}

	// Create and broadcast transactions
	for i, txData := range transactions {
		tx := &Transaction{
			From:      n.address,
			To:        txData.to,
			Value:     txData.value,
			Nonce:     txData.nonce,
			Fee:       1,
			Timestamp: time.Now().UnixNano(),
			Type:      txData.txType,
		}

		// Sign the transaction
		if err := tx.Sign(n.privKey); err != nil {
			log.Printf("ERROR: Failed to sign test transaction: %v", err)
			continue
		}

		// Broadcast the transaction
		if err := n.BroadcastTransaction(tx); err != nil {
			log.Printf("ERROR: Failed to broadcast test transaction: %v", err)
		} else {
			log.Printf("INFO: Broadcast test transaction %d with hash %s (value: %d)", i, tx.Hash.ToHex(), txData.value)
		}

		// Wait a bit between transactions
		time.Sleep(1 * time.Second)
	}

	// Add some delegation transactions if we have other validators
	validators, err := n.vr.GetAllValidators()
	if err == nil && len(validators) > 1 {
		log.Printf("INFO: Creating delegation transactions...")

		// Find another validator to delegate to
		for _, v := range validators {
			if v.Address != n.address {
				// Create delegation transaction
				delegationTx := &Transaction{
					From:      n.address,
					To:        v.Address,
					Value:     50, // Delegate 50 tokens
					Nonce:     6,
					Fee:       1,
					Timestamp: time.Now().UnixNano(),
					Type:      "delegate",
				}

				if err := delegationTx.Sign(n.privKey); err != nil {
					log.Printf("ERROR: Failed to sign delegation transaction: %v", err)
					continue
				}

				if err := n.BroadcastTransaction(delegationTx); err != nil {
					log.Printf("ERROR: Failed to broadcast delegation transaction: %v", err)
				} else {
					log.Printf("INFO: Broadcast delegation transaction to %s with hash %s", v.Address.ToHex(), delegationTx.Hash.ToHex())
				}
				break
			}
		}
	}
}
