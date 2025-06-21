package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"golang.org/x/crypto/sha3"
)

const (
	TransactionTopic = "/dyphira/transactions/v1"
	BlockTopic       = "/dyphira/blocks/v1"
	ApprovalTopic    = "/dyphira/approvals/v1"
	EpochLength      = 10 // blocks
)

// AppNode represents the full blockchain application.
type AppNode struct {
	p2p     *P2PNode
	bc      *Blockchain
	state   *State
	txPool  *TransactionPool
	vr      *ValidatorRegistry
	ctx     context.Context
	privKey *ecdsa.PrivateKey
	address Address

	// DPoS / Consensus
	committee        []*Validator
	proposerSelector *ProposerSelector
	pendingBlocks    map[Hash]*BlockApproval
	pendingBlocksMu  sync.RWMutex

	// Buffer for approvals received before the block
	approvalBuffer   map[Hash][]*Approval
	approvalBufferMu sync.Mutex

	// Inactivity tracking for committee members
	inactivity map[Address]int
}

// --- Add a global test hook for committee sync (for tests only) ---
var TestSyncCommittee func(newCommittee []*Validator)

// NewAppNode creates and initializes a new full blockchain node.
func NewAppNode(ctx context.Context, listenPort int, dbPath string, p2pPrivKey crypto.PrivKey, privKey *ecdsa.PrivateKey) (*AppNode, error) {
	// --- Storage ---
	chainStore, err := NewBoltStore(dbPath, "chain")
	if err != nil {
		return nil, fmt.Errorf("failed to open chain store: %w", err)
	}
	validatorStore, err := NewBoltStore(dbPath, "validators")
	if err != nil {
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

	// --- P2P Component ---
	p2p, err := NewP2PNode(ctx, listenPort, p2pPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create P2P node: %w", err)
	}

	// Use the libp2p public key to derive the blockchain address
	pubKeyBytes, err := p2pPrivKey.GetPublic().Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}
	var addr Address
	copy(addr[:], pubKeyBytes[:20])

	node := &AppNode{
		p2p:            p2p,
		bc:             bc,
		state:          state,
		txPool:         txPool,
		vr:             vr,
		ctx:            ctx,
		privKey:        privKey,
		address:        addr,
		pendingBlocks:  make(map[Hash]*BlockApproval),
		approvalBuffer: make(map[Hash][]*Approval),
		inactivity:     make(map[Address]int),
	}

	// Register self as a validator
	if err := vr.RegisterValidator(&Validator{Address: addr, Stake: 100, Participating: true}); err != nil {
		return nil, fmt.Errorf("failed to register self as validator: %w", err)
	}
	log.Printf("Node %s registered as validator", addr.ToHex())
	return node, nil
}

// NewAppNodeWithStores creates a node with explicit chain and validator stores (for testing).
func NewAppNodeWithStores(ctx context.Context, listenPort int, p2pPrivKey crypto.PrivKey, privKey *ecdsa.PrivateKey, chainStore *BoltStore, validatorStore *BoltStore) (*AppNode, error) {
	// --- Blockchain Components ---
	bc, err := NewBlockchain(chainStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create blockchain: %w", err)
	}
	state := NewState()
	vr := NewValidatorRegistry(validatorStore, "validators")
	txPool := NewTransactionPool()

	// --- P2P Component ---
	p2p, err := NewP2PNode(ctx, listenPort, p2pPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create P2P node: %w", err)
	}

	// Use the libp2p public key to derive the blockchain address
	pubKeyBytes, err := p2pPrivKey.GetPublic().Raw()
	if err != nil {
		return nil, fmt.Errorf("failed to get public key: %w", err)
	}
	var addr Address
	copy(addr[:], pubKeyBytes[:20])

	node := &AppNode{
		p2p:            p2p,
		bc:             bc,
		state:          state,
		txPool:         txPool,
		vr:             vr,
		ctx:            ctx,
		privKey:        privKey,
		address:        addr,
		pendingBlocks:  make(map[Hash]*BlockApproval),
		approvalBuffer: make(map[Hash][]*Approval),
		inactivity:     make(map[Address]int),
	}

	// Register self as a validator
	if err := vr.RegisterValidator(&Validator{Address: addr, Stake: 100, Participating: true}); err != nil {
		return nil, fmt.Errorf("failed to register self as validator: %w", err)
	}
	log.Printf("Node %s registered as validator", addr.ToHex())
	return node, nil
}

// Close gracefully shuts down the node.
func (n *AppNode) Close() error {
	// Close the p2p host, blockchain db, etc.
	if err := n.p2p.host.Close(); err != nil {
		return fmt.Errorf("failed to close p2p host: %w", err)
	}
	if err := n.bc.Close(); err != nil {
		return fmt.Errorf("failed to close blockchain: %w", err)
	}
	return nil
}

// Start begins the node's operation.
func (n *AppNode) Start() error {
	n.p2p.RegisterTopic(TransactionTopic)
	n.p2p.RegisterTopic(BlockTopic)
	n.p2p.RegisterTopic(ApprovalTopic)

	go n.p2p.Subscribe(n.ctx, n.handleNetworkMessage)
	go n.p2p.Discover(n.ctx)
	go n.producerLoop()

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
	}
}

func (n *AppNode) handleTransaction(msg *pubsub.Message) {
	var netTx NetworkTransaction
	if err := json.Unmarshal(msg.Data, &netTx); err != nil {
		log.Printf("Failed to decode network tx: %v", err)
		return
	}
	if err := n.txPool.AddTransaction(netTx.Tx, netTx.PubKey, n.state); err != nil {
		if !strings.Contains(err.Error(), "already in pool") {
			log.Printf("Failed to add tx %s from network: %v", netTx.Tx.Hash.ToHex(), err)
		}
	} else {
		log.Printf("Added transaction %s to pool", netTx.Tx.Hash.ToHex())
	}
}

func (n *AppNode) handleBlockProposal(msg *pubsub.Message) {
	var block Block
	if err := json.Unmarshal(msg.Data, &block); err != nil {
		log.Printf("ERROR: Failed to decode block proposal: %v", err)
		return
	}
	log.Printf("INFO: Node %s received block proposal #%d from %s", n.address.ToHex(), block.Header.BlockNumber, block.Header.Proposer.ToHex())

	// TODO: Add block validation (check proposer, etc.)

	n.pendingBlocksMu.Lock()
	if _, exists := n.pendingBlocks[block.Header.Hash]; exists {
		log.Printf("WARN: Node %s already saw block %s. Ignoring.", n.address.ToHex(), block.Header.Hash.ToHex())
		n.pendingBlocksMu.Unlock()
		return // Already seen
	}
	ba := NewBlockApproval(&block, n.committee)
	n.pendingBlocks[block.Header.Hash] = ba
	n.pendingBlocksMu.Unlock()

	// --- Process any buffered approvals for this block ---
	n.approvalBufferMu.Lock()
	buffered := n.approvalBuffer[block.Header.Hash]
	delete(n.approvalBuffer, block.Header.Hash)
	n.approvalBufferMu.Unlock()
	for _, approval := range buffered {
		n.processApproval(approval)
	}

	// Start a timer to handle block approval timeout.
	go n.watchBlockApproval(&block)

	// If this node is on the committee, vote.
	for _, member := range n.committee {
		if member.Address == n.address {
			log.Printf("INFO: Node %s (committee member) is voting for block #%d", n.address.ToHex(), block.Header.BlockNumber)
			n.broadcastApproval(&block)
			break
		}
	}

	// Apply block to state and update participation
	if err := n.bc.ApplyBlockWithRegistry(&block, n.state, n.vr); err != nil {
		log.Printf("ERROR: Failed to apply block: %v", err)
	}
}

func (n *AppNode) handleApproval(msg *pubsub.Message) {
	var approval Approval
	if err := json.Unmarshal(msg.Data, &approval); err != nil {
		log.Printf("ERROR: Failed to decode approval: %v", err)
		return
	}
	log.Printf("INFO: Node %s received approval for block %s from %s", n.address.ToHex(), approval.BlockHash.ToHex(), approval.Address.ToHex())

	n.pendingBlocksMu.Lock()
	exists := n.pendingBlocks[approval.BlockHash] != nil
	n.pendingBlocksMu.Unlock()
	if !exists {
		// Buffer the approval for later
		n.approvalBufferMu.Lock()
		n.approvalBuffer[approval.BlockHash] = append(n.approvalBuffer[approval.BlockHash], &approval)
		n.approvalBufferMu.Unlock()
		log.Printf("WARN: Node %s buffered approval for unknown block %s.", n.address.ToHex(), approval.BlockHash.ToHex())
		return
	}

	n.processApproval(&approval)
}

// processApproval processes an approval for a known block (must be called with ba != nil)
func (n *AppNode) processApproval(approval *Approval) {
	n.pendingBlocksMu.Lock()
	ba, exists := n.pendingBlocks[approval.BlockHash]
	n.pendingBlocksMu.Unlock()
	if !exists {
		return
	}

	if ba.IsApproved() {
		log.Printf("INFO: Node %s noted approval for block %s, but it's already approved.", n.address.ToHex(), approval.BlockHash.ToHex())
		return // Already approved and processed
	}

	if err := ba.AddSignature(approval.Address, approval.Signature); err != nil {
		log.Printf("ERROR: Node %s failed to add signature for block %s: %v", n.address.ToHex(), approval.BlockHash.ToHex(), err)
		return
	}
	log.Printf("INFO: Node %s added approval for block %s from %s. Total approvals: %d/%d", n.address.ToHex(), approval.BlockHash.ToHex(), approval.Address.ToHex(), len(ba.Signatures), ba.Threshold)

	if ba.IsApproved() {
		log.Printf("SUCCESS: Node %s confirms block %s is now APPROVED!", n.address.ToHex(), ba.Block.Header.Hash.ToHex())
		if err := n.bc.AddBlock(ba.Block); err != nil {
			log.Printf("CRITICAL: Node %s failed to add approved block %d to chain: %v", n.address.ToHex(), ba.Block.Header.BlockNumber, err)
		} else {
			log.Printf("SUCCESS: Node %s added approved block %d to blockchain.", n.address.ToHex(), ba.Block.Header.BlockNumber)
			// Clean up pending block
			n.pendingBlocksMu.Lock()
			delete(n.pendingBlocks, approval.BlockHash)
			n.pendingBlocksMu.Unlock()
		}
	}
}

// watchBlockApproval waits for a block to be approved and cleans it up if it times out.
func (n *AppNode) watchBlockApproval(block *Block) {
	select {
	case <-time.After(250 * time.Millisecond):
		n.pendingBlocksMu.Lock()
		defer n.pendingBlocksMu.Unlock()

		ba, exists := n.pendingBlocks[block.Header.Hash]
		if !exists {
			log.Printf("DEBUG: Watchdog for block %s found it was already removed (likely approved).", block.Header.Hash.ToHex())
			return
		}

		if !ba.IsApproved() {
			log.Printf("WARN: Block #%d from %s timed out without approval on node %s. Removing.", block.Header.BlockNumber, block.Header.Proposer.ToHex(), n.address.ToHex())
			delete(n.pendingBlocks, block.Header.Hash)

			// --- Inactivity tracking and replacement ---
			approvers := make(map[Address]bool)
			for addrHex := range ba.Signatures {
				for _, v := range n.committee {
					if v.Address.ToHex() == addrHex {
						approvers[v.Address] = true
					}
				}
			}
			for _, v := range n.committee {
				if !approvers[v.Address] {
					n.inactivity[v.Address]++
					if n.inactivity[v.Address] > 1 {
						log.Printf("INFO: Validator %s is inactive and will be replaced.", v.Address.ToHex())
						newCommittee, err := ReplaceInactiveValidator(n.committee, v.Address, n.vr)
						if err != nil {
							log.Printf("ERROR: could not replace inactive validator %s: %v", v.Address.ToHex(), err)
						} else {
							n.committee = newCommittee
							n.proposerSelector = NewProposerSelectorWithRotation(newCommittee, n.bc.Height(), EpochLength)
						}
					}
				}
			}

			// --- Empty block production on timeout ---
			if n.proposerSelector != nil {
				proposer := n.proposerSelector.ProposerForBlock(block.Header.BlockNumber)
				if proposer != nil && proposer.Address == n.address {
					log.Printf("INFO: Node %s is producing EMPTY block #%d due to timeout.", n.address.ToHex(), block.Header.BlockNumber)
					emptyBlock, err := n.bc.CreateBlock([]*Transaction{}, proposer, n.privKey)
					if err != nil {
						log.Printf("ERROR: Node %s failed to create empty block: %v", n.address.ToHex(), err)
						return
					}
					blockBytes, err := json.Marshal(emptyBlock)
					if err != nil {
						log.Printf("ERROR: Node %s failed to marshal empty block: %v", n.address.ToHex(), err)
						return
					}
					if err := n.p2p.Publish(n.ctx, BlockTopic, blockBytes); err != nil {
						log.Printf("ERROR: Node %s failed to publish empty block: %v", n.address.ToHex(), err)
					} else {
						log.Printf("SUCCESS: Node %s published EMPTY block #%d", n.address.ToHex(), emptyBlock.Header.BlockNumber)
					}
				}
			}
		}
	case <-n.ctx.Done():
		// Node is shutting down.
	}
}

// --- Broadcasting ---

func (n *AppNode) BroadcastTransaction(tx *Transaction) error {
	tx.From = n.address
	data, err := tx.Encode()
	if err != nil {
		return err
	}
	tx.Hash = sha3.Sum256(data)

	r, s, err := ecdsa.Sign(rand.Reader, n.privKey, tx.Hash[:])
	if err != nil {
		return err
	}
	tx.R, tx.S = r.Bytes(), s.Bytes()

	netTx := &NetworkTransaction{Tx: tx, PubKey: &n.privKey.PublicKey}
	txBytes, err := json.Marshal(netTx)
	if err != nil {
		return err
	}

	log.Printf("Broadcasting transaction: %s", tx.Hash.ToHex())
	return n.p2p.Publish(n.ctx, TransactionTopic, txBytes)
}

func (n *AppNode) broadcastApproval(block *Block) {
	r, s, err := ecdsa.Sign(rand.Reader, n.privKey, block.Header.Hash[:])
	if err != nil {
		log.Printf("Failed to sign approval: %v", err)
		return
	}
	sig := append(r.Bytes(), s.Bytes()...)

	approval := &Approval{
		BlockHash: block.Header.Hash,
		Address:   n.address,
		Signature: sig,
	}
	approvalBytes, err := json.Marshal(approval)
	if err != nil {
		log.Printf("Failed to marshal approval: %v", err)
		return
	}
	if err := n.p2p.Publish(n.ctx, ApprovalTopic, approvalBytes); err != nil {
		log.Printf("Failed to publish approval: %v", err)
	}
}

// --- Main Loop ---

func (n *AppNode) producerLoop() {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			currentHeight := n.bc.Height()
			if n.committee == nil || currentHeight%EpochLength == 0 {
				committee, err := (&CommitteeSelector{Registry: n.vr}).SelectCommittee(5)
				if err != nil {
					log.Printf("ERROR: Node %s failed to select committee: %v", n.address.ToHex(), err)
					continue
				}
				n.committee = committee
				n.proposerSelector = NewProposerSelectorWithRotation(committee, currentHeight, EpochLength)
				log.Printf("INFO: Node %s elected new committee for epoch starting at height %d. Committee size: %d", n.address.ToHex(), currentHeight, len(committee))
			}

			if n.proposerSelector == nil || len(n.proposerSelector.Committee) == 0 {
				log.Printf("WARN: Node %s has no proposer selector or empty committee. Skipping block production.", n.address.ToHex())
				continue
			}

			proposer := n.proposerSelector.ProposerForBlock(currentHeight)
			if proposer != nil && proposer.Address == n.address {
				log.Printf("INFO: Node %s IS the proposer. Producing block...", n.address.ToHex())
				txs := n.txPool.SelectTransactions(10)

				newBlock, err := n.bc.CreateBlock(txs, proposer, n.privKey)
				if err != nil {
					log.Printf("ERROR: Node %s failed to create block: %v", n.address.ToHex(), err)
					continue
				}

				blockBytes, err := json.Marshal(newBlock)
				if err != nil {
					log.Printf("ERROR: Node %s failed to marshal block: %v", n.address.ToHex(), err)
					continue
				}
				log.Printf("INFO: Node %s broadcasting block proposal #%d", n.address.ToHex(), newBlock.Header.BlockNumber)
				if err := n.p2p.Publish(n.ctx, BlockTopic, blockBytes); err != nil {
					log.Printf("ERROR: Node %s failed to publish block proposal: %v", n.address.ToHex(), err)
				} else {
					log.Printf("SUCCESS: Node %s published block proposal #%d", n.address.ToHex(), newBlock.Header.BlockNumber)
				}
			}
		case <-n.ctx.Done():
			return
		}
	}
}

func (n *AppNode) ForceCommitteeAndProposer() {
	currentHeight := n.bc.Height()
	committee, err := (&CommitteeSelector{Registry: n.vr}).SelectCommittee(5)
	if err != nil {
		log.Printf("ERROR: Node %s failed to select committee: %v", n.address.ToHex(), err)
		return
	}
	n.committee = committee
	n.proposerSelector = NewProposerSelectorWithRotation(committee, currentHeight, EpochLength)
	log.Printf("INFO: Node %s (force) elected committee for epoch starting at height %d. Committee size: %d", n.address.ToHex(), currentHeight, len(committee))
}

// ReplaceInactiveValidator removes an inactive validator from the committee and adds a new one from the registry.
func ReplaceInactiveValidator(committee []*Validator, inactiveAddress Address, vr *ValidatorRegistry) ([]*Validator, error) {
	// Find and remove the inactive validator
	newCommittee := make([]*Validator, 0, len(committee)-1)
	for _, v := range committee {
		if v.Address != inactiveAddress {
			newCommittee = append(newCommittee, v)
		}
	}

	// Select a new validator from the registry
	allValidators, err := vr.ListValidators()
	if err != nil {
		return nil, fmt.Errorf("failed to list validators: %w", err)
	}

	// Create a lookup map of validators already in the new committee
	inCommittee := make(map[Address]bool)
	for _, v := range newCommittee {
		inCommittee[v.Address] = true
	}

	// Find a replacement that is not already in the committee
	var newValidator *Validator
	for _, val := range allValidators {
		if !inCommittee[val.Address] {
			newValidator = val
			break
		}
	}

	if newValidator == nil {
		return nil, fmt.Errorf("no valid validator found to replace %s", inactiveAddress.ToHex())
	}

	newCommittee = append(newCommittee, newValidator)
	return newCommittee, nil
}
