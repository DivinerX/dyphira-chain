package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// BAR Network Constants
const (
	DefaultOutgoingConnections = 8
	DefaultIncomingConnections = 8
	DefaultSyncTimeout         = 10
	DefaultPOMThreshold        = 5
	DefaultPOMBanThreshold     = 15
	DefaultReputationMemory    = 10
	DefaultHandshakeTimeout    = 5
)

// PeerStatus represents the status of a peer in the BAR network
type PeerStatus int

const (
	PeerStatusGreylist PeerStatus = iota
	PeerStatusWhitelist
	PeerStatusBanned
)

// PeerInfo contains information about a peer in the BAR network
type PeerInfo struct {
	ID            peer.ID
	Address       string
	Status        PeerStatus
	POMScore      int
	LastSeen      time.Time
	HandshakeAt   time.Time
	Reputation    int
	IsSeed        bool
	ClientVersion string
	LastRound     uint64
}

// ReputationRecord tracks POM scores for peers
type ReputationRecord struct {
	POMScore  int
	LastRound uint64
	Reason    string
}

// BARNetwork implements the BAR Resilient Network model
type BARNetwork struct {
	mu sync.RWMutex

	// Peer management
	whitelist map[peer.ID]*PeerInfo
	greylist  map[peer.ID]*PeerInfo
	banned    map[peer.ID]*PeerInfo

	// Configuration
	config *BARConfig

	// Reputation tracking
	reputationRecords map[peer.ID]*ReputationRecord

	// PRNG for find_nodes algorithm
	prngSeed []byte
	round    uint64

	// Seed nodes
	seedNodes []string

	// Callbacks
	onPeerStatusChange func(peerID peer.ID, oldStatus, newStatus PeerStatus)
}

// BARConfig contains configuration for the BAR network
type BARConfig struct {
	OutgoingConnections int
	IncomingConnections int
	SyncTimeout         time.Duration
	POMThreshold        int
	POMBanThreshold     int
	ReputationMemory    int
	HandshakeTimeout    time.Duration
	SeedNodes           []string
}

// NewBARNetwork creates a new BAR network instance
func NewBARNetwork(config *BARConfig) *BARNetwork {
	if config == nil {
		config = &BARConfig{
			OutgoingConnections: DefaultOutgoingConnections,
			IncomingConnections: DefaultIncomingConnections,
			SyncTimeout:         DefaultSyncTimeout * time.Second,
			POMThreshold:        DefaultPOMThreshold,
			POMBanThreshold:     DefaultPOMBanThreshold,
			ReputationMemory:    DefaultReputationMemory,
			HandshakeTimeout:    DefaultHandshakeTimeout * time.Second,
		}
	}

	// Generate initial PRNG seed
	seed := make([]byte, 32)
	rand.Read(seed)

	return &BARNetwork{
		whitelist:         make(map[peer.ID]*PeerInfo),
		greylist:          make(map[peer.ID]*PeerInfo),
		banned:            make(map[peer.ID]*PeerInfo),
		config:            config,
		reputationRecords: make(map[peer.ID]*ReputationRecord),
		prngSeed:          seed,
		round:             0,
		seedNodes:         config.SeedNodes,
	}
}

// AddPeer adds a peer to the greylist (initial state for new peers)
func (bn *BARNetwork) AddPeer(peerID peer.ID, address string) {
	bn.mu.Lock()
	defer bn.mu.Unlock()

	// Don't add if already in any list
	if bn.isPeerInAnyList(peerID) {
		return
	}

	peerInfo := &PeerInfo{
		ID:       peerID,
		Address:  address,
		Status:   PeerStatusGreylist,
		POMScore: 0,
		LastSeen: time.Now(),
	}

	bn.greylist[peerID] = peerInfo
	log.Printf("BAR: Added peer %s to greylist", peerID)
}

// PromoteToWhitelist promotes a peer from greylist to whitelist
func (bn *BARNetwork) PromoteToWhitelist(peerID peer.ID) bool {
	bn.mu.Lock()
	defer bn.mu.Unlock()
	return bn.promoteToWhitelistNoLock(peerID)
}

// promoteToWhitelistNoLock promotes a peer from greylist to whitelist (no lock)
func (bn *BARNetwork) promoteToWhitelistNoLock(peerID peer.ID) bool {
	peerInfo, exists := bn.greylist[peerID]
	if !exists {
		return false
	}
	// Check if we have room in whitelist
	if len(bn.whitelist) >= bn.config.IncomingConnections {
		log.Printf("BAR: Cannot promote peer %s - whitelist full", peerID)
		return false
	}
	// Move from greylist to whitelist
	delete(bn.greylist, peerID)
	peerInfo.Status = PeerStatusWhitelist
	peerInfo.HandshakeAt = time.Now()
	bn.whitelist[peerID] = peerInfo
	log.Printf("BAR: Promoted peer %s to whitelist", peerID)
	return true
}

// DemoteToGreylist demotes a peer from whitelist to greylist
func (bn *BARNetwork) DemoteToGreylist(peerID peer.ID) bool {
	bn.mu.Lock()
	defer bn.mu.Unlock()
	return bn.demoteToGreylistNoLock(peerID)
}

// demoteToGreylistNoLock demotes a peer from whitelist to greylist (no lock)
func (bn *BARNetwork) demoteToGreylistNoLock(peerID peer.ID) bool {
	peerInfo, exists := bn.whitelist[peerID]
	if !exists {
		return false
	}
	// Move from whitelist to greylist
	delete(bn.whitelist, peerID)
	peerInfo.Status = PeerStatusGreylist
	bn.greylist[peerID] = peerInfo
	log.Printf("BAR: Demoted peer %s to greylist", peerID)
	return true
}

// BanPeer bans a peer from the network
func (bn *BARNetwork) BanPeer(peerID peer.ID, reason string) {
	bn.mu.Lock()
	defer bn.mu.Unlock()
	bn.banPeerNoLock(peerID, reason)
}

// banPeerNoLock bans a peer from the network (no lock)
func (bn *BARNetwork) banPeerNoLock(peerID peer.ID, reason string) {
	// Remove from whitelist or greylist
	var peerInfo *PeerInfo
	if info, exists := bn.whitelist[peerID]; exists {
		delete(bn.whitelist, peerID)
		peerInfo = info
	} else if info, exists := bn.greylist[peerID]; exists {
		delete(bn.greylist, peerID)
		peerInfo = info
	} else {
		return // Already banned or not found
	}

	// Add to banned list
	peerInfo.Status = PeerStatusBanned
	bn.banned[peerID] = peerInfo

	// Record reputation
	bn.reputationRecords[peerID] = &ReputationRecord{
		POMScore:  peerInfo.POMScore,
		LastRound: bn.round,
		Reason:    reason,
	}

	log.Printf("BAR: Banned peer %s: %s", peerID, reason)
}

// UpdatePOMScore updates the POM score for a peer
func (bn *BARNetwork) UpdatePOMScore(peerID peer.ID, increment int, reason string) {
	bn.mu.Lock()
	defer bn.mu.Unlock()

	// Find peer in whitelist or greylist
	var peerInfo *PeerInfo
	if info, exists := bn.whitelist[peerID]; exists {
		peerInfo = info
	} else if info, exists := bn.greylist[peerID]; exists {
		peerInfo = info
	} else {
		return // Peer not found
	}

	oldScore := peerInfo.POMScore
	peerInfo.POMScore += increment
	peerInfo.LastSeen = time.Now()

	log.Printf("BAR: Updated POM score for peer %s: %d -> %d (%s)",
		peerID, oldScore, peerInfo.POMScore, reason)

	// Check if peer should be demoted or banned
	if peerInfo.Status == PeerStatusWhitelist && peerInfo.POMScore >= bn.config.POMThreshold {
		bn.demoteToGreylistNoLock(peerID)
	} else if peerInfo.Status == PeerStatusGreylist && peerInfo.POMScore >= bn.config.POMBanThreshold {
		bn.banPeerNoLock(peerID, fmt.Sprintf("POM score %d exceeded ban threshold", peerInfo.POMScore))
	}
}

// FindNodes implements the find_nodes algorithm using PRNG
func (bn *BARNetwork) FindNodes(round uint64) []peer.ID {
	bn.mu.RLock()
	defer bn.mu.RUnlock()

	// Update round
	bn.round = round

	// Generate PRNG seed for this round
	roundSeed := bn.generateRoundSeed(round)

	// Get eligible peers from whitelist
	eligiblePeers := make([]*PeerInfo, 0, len(bn.whitelist))
	for _, peer := range bn.whitelist {
		eligiblePeers = append(eligiblePeers, peer)
	}

	if len(eligiblePeers) == 0 {
		return []peer.ID{}
	}

	// Use PRNG to select peers
	selectedPeers := make([]peer.ID, 0, bn.config.IncomingConnections)
	used := make(map[int]bool)

	for i := 0; i < bn.config.IncomingConnections && i < len(eligiblePeers); i++ {
		// Generate random index using round seed
		index := bn.prngSelect(roundSeed, i, len(eligiblePeers))

		// Find next available index if this one is used
		for used[index] {
			index = (index + 1) % len(eligiblePeers)
		}

		used[index] = true
		selectedPeers = append(selectedPeers, eligiblePeers[index].ID)
	}

	log.Printf("BAR: Selected %d peers for round %d using find_nodes algorithm",
		len(selectedPeers), round)

	return selectedPeers
}

// generateRoundSeed generates a deterministic seed for a given round
func (bn *BARNetwork) generateRoundSeed(round uint64) []byte {
	// Combine base seed with round number
	roundBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(roundBytes, round)

	combined := append(bn.prngSeed, roundBytes...)
	hash := sha256.Sum256(combined)
	return hash[:]
}

// prngSelect selects a random index using the round seed
func (bn *BARNetwork) prngSelect(seed []byte, iteration, max int) int {
	// Use seed + iteration to generate deterministic random number
	iterBytes := make([]byte, 8)
	binary.BigEndian.PutUint64(iterBytes, uint64(iteration))

	combined := append(seed, iterBytes...)
	hash := sha256.Sum256(combined)

	// Use first 4 bytes to generate random number
	randomNum := binary.BigEndian.Uint32(hash[:4])
	return int(randomNum % uint32(max))
}

// GetWhitelist returns all peers in the whitelist
func (bn *BARNetwork) GetWhitelist() []*PeerInfo {
	bn.mu.RLock()
	defer bn.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(bn.whitelist))
	for _, peer := range bn.whitelist {
		peers = append(peers, peer)
	}
	return peers
}

// GetGreylist returns all peers in the greylist
func (bn *BARNetwork) GetGreylist() []*PeerInfo {
	bn.mu.RLock()
	defer bn.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(bn.greylist))
	for _, peer := range bn.greylist {
		peers = append(peers, peer)
	}
	return peers
}

// GetBanned returns all banned peers
func (bn *BARNetwork) GetBanned() []*PeerInfo {
	bn.mu.RLock()
	defer bn.mu.RUnlock()

	peers := make([]*PeerInfo, 0, len(bn.banned))
	for _, peerInfo := range bn.banned {
		peers = append(peers, peerInfo)
	}
	return peers
}

// GetAllPeers returns all peers from all lists (whitelist, greylist, banned)
func (bn *BARNetwork) GetAllPeers() []*PeerInfo {
	bn.mu.RLock()
	defer bn.mu.RUnlock()

	peers := make([]*PeerInfo, 0)

	// Add whitelist peers
	for _, peerInfo := range bn.whitelist {
		peers = append(peers, peerInfo)
	}

	// Add greylist peers
	for _, peerInfo := range bn.greylist {
		peers = append(peers, peerInfo)
	}

	// Add banned peers
	for _, peerInfo := range bn.banned {
		peers = append(peers, peerInfo)
	}

	return peers
}

// GetPeerStatus returns the current status of a peer
func (bn *BARNetwork) GetPeerStatus(peerID peer.ID) (PeerStatus, bool) {
	bn.mu.RLock()
	defer bn.mu.RUnlock()

	if peerInfo, exists := bn.whitelist[peerID]; exists {
		return peerInfo.Status, true
	}
	if peerInfo, exists := bn.greylist[peerID]; exists {
		return peerInfo.Status, true
	}
	if peerInfo, exists := bn.banned[peerID]; exists {
		return peerInfo.Status, true
	}
	return PeerStatusGreylist, false
}

// SetOnPeerStatusChange sets the callback for peer status changes
func (bn *BARNetwork) SetOnPeerStatusChange(callback func(peerID peer.ID, oldStatus, newStatus PeerStatus)) {
	bn.mu.Lock()
	defer bn.mu.Unlock()
	bn.onPeerStatusChange = callback
}

// isPeerInAnyList checks if a peer is in any list
func (bn *BARNetwork) isPeerInAnyList(peerID peer.ID) bool {
	_, exists := bn.whitelist[peerID]
	if exists {
		return true
	}
	_, exists = bn.greylist[peerID]
	if exists {
		return true
	}
	_, exists = bn.banned[peerID]
	return exists
}

// CleanupReputationRecords removes old reputation records
func (bn *BARNetwork) CleanupReputationRecords() {
	bn.mu.Lock()
	defer bn.mu.Unlock()

	currentRound := bn.round
	for peerID, record := range bn.reputationRecords {
		if currentRound-record.LastRound > uint64(bn.config.ReputationMemory) {
			delete(bn.reputationRecords, peerID)
		}
	}
}
