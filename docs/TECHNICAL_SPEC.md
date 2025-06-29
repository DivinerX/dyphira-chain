# Dyphira L1 Technical Specification

## Overview

Dyphira L1 is a Delegated Proof-of-Stake (DPoS) blockchain implementation that provides a robust, scalable, and fault-tolerant consensus mechanism. This document outlines the technical specifications, architecture, and implementation details.

## New Features (2024)

### Fast Sync Protocol
- **Purpose**: Rapidly synchronizes new nodes to the latest block height/state.
- **Protocol**: Node requests state snapshot and block stream from peers, applies them, and switches to normal sync when caught up.
- **Manager**: `FastSyncManager` handles sync logic and peer coordination.

### Transaction Batching
- **Purpose**: Groups transactions into batches for efficient block production.
- **Batcher**: `TransactionBatcher` manages batches, configurable size/timeout, and exposes batch metrics.
- **Metrics**: Batch size, processing time, throughput, and success rate.

### Enhanced Metrics
- **Purpose**: Collects and exports node, network, consensus, and performance metrics.
- **Collector**: `MetricsCollector` periodically collects and exports metrics as JSON.
- **Schema**: Includes peer count, block height, committee size, approval rate, tx pool size, memory usage, goroutines, syncing status, and more.

### Graceful Shutdown
- **Purpose**: Ensures all node components shut down cleanly and in the correct order.
- **Manager**: `GracefulShutdown` coordinates shutdown, handles OS signals, and reports status/reason.

## Architecture

### High-Level Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   P2P Network   │    │   Consensus     │    │   State Mgmt    │
│                 │    │                 │    │                 │
│ • libp2p        │    │ • DPoS          │    │ • Merkle Trie   │
│ • GossipSub     │    │ • Committees    │    │ • Accounts      │
│ • DHT           │    │ • Block Approval│    │ • Transactions  │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         └───────────────────────┼───────────────────────┘
                                 │
                    ┌─────────────────┐
                    │   Storage       │
                    │                 │
                    │ • BoltDB        │
                    │ • In-Memory     │
                    └─────────────────┘
```

### Component Interaction

1. **P2P Layer**: Handles peer discovery, message propagation, and network connectivity
2. **Consensus Layer**: Manages DPoS consensus, committee selection, and block approval
3. **State Layer**: Maintains account states, transaction processing, and state transitions
4. **Storage Layer**: Provides persistent and in-memory storage for blocks and state

## Consensus Protocol

### DPoS Overview

The DPoS consensus protocol operates in epochs, where each epoch consists of 10 blocks. Within each epoch:

1. **Committee Selection**: A committee of validators is selected based on stake, delegation, and participation
2. **Proposer Rotation**: Block proposers are rotated round-robin within the committee
3. **Block Approval**: Blocks require 2/3 committee approval for finalization

### Committee Selection Algorithm

```go
func (c *Committee) SelectCommittee(validators []Validator, epoch uint64) []Validator {
    // Filter participating validators
    participating := filterParticipating(validators)
    
    // Sort by total stake (own stake + delegations)
    sort.Sort(ByTotalStake(participating))
    
    // Select top validators (committee size based on network size)
    committeeSize := min(len(participating), maxCommitteeSize)
    return participating[:committeeSize]
}
```

### Block Production and Approval

1. **Block Proposal**: Current proposer creates and broadcasts a block
2. **Transaction Inclusion**: Valid transactions from the pool are included
3. **Committee Signing**: Committee members sign the block using Secp256k1 ECDSA
4. **Finalization**: Block is finalized when 2/3 threshold is reached

## Transaction System

### Transaction Types

#### 1. Transfer Transaction
```go
{
    "type": "transfer",
    "from": "dyphira1...", // BECH-32 encoded address
    "to": "dyphira1...",   // BECH-32 encoded address
    "value": 100,
    "nonce": 1,
    "fee": 1,
    "signature": "..." // ASN.1-encoded ECDSA signature
}
```

#### 2. Validator Registration
```go
{
    "type": "register_validator",
    "from": "dyphira1...", // BECH-32 encoded address
    "to": "dyphira1...",   // Same as from
    "value": 100,          // Stake amount
    "nonce": 1,
    "fee": 1,
    "signature": "..." // ASN.1-encoded ECDSA signature
}
```

#### 3. Delegation Transaction
```go
{
    "type": "delegate",
    "from": "dyphira1...", // BECH-32 encoded address
    "to": "dyphira1...",   // Validator address
    "value": 50,           // Delegation amount
    "nonce": 1,
    "fee": 1,
    "signature": "..." // ASN.1-encoded ECDSA signature
}
```

#### 4. Participation Transaction
```go
{
    "type": "participation",
    "from": "dyphira1...", // BECH-32 encoded address
    "to": "dyphira1...",   // Same as from
    "value": 0,            // Not used
    "nonce": 1,
    "fee": 1,
    "signature": "..." // ASN.1-encoded ECDSA signature
}
```

### Transaction Validation

```go
func (tp *TransactionPool) ValidateTransaction(tx *Transaction) error {
    // Check Secp256k1 signature
    if !tx.VerifySignature() {
        return errors.New("invalid signature")
    }
    
    // Check nonce
    if tx.Nonce <= accountNonce {
        return errors.New("invalid nonce")
    }
    
    // Check balance
    if accountBalance < tx.Value + tx.Fee {
        return errors.New("insufficient balance")
    }
    
    // Type-specific validation
    switch tx.Type {
    case "register_validator":
        return validateValidatorRegistration(tx)
    case "delegate":
        return validateDelegation(tx)
    }
    
    return nil
}
```

## State Management

### Merkle Trie Implementation

The state is managed using a Merkle Trie for efficient state proofs and updates:

```go
type MerkleTrie struct {
    root *Node
    db   Storage
}

type Node struct {
    hash     Hash
    children map[byte]*Node
    value    []byte
    isLeaf   bool
}
```

### Account State

```go
type Account struct {
    Balance    uint64            `json:"balance"`
    Nonce      uint64            `json:"nonce"`
    Delegations map[Address]uint64 `json:"delegations"`
    IsValidator bool             `json:"is_validator"`
    Participating bool           `json:"participating"`
}
```

### State Transitions

```go
func (s *State) ApplyTransaction(tx *Transaction) error {
    // Deduct fee and value from sender
    s.DeductBalance(tx.From, tx.Value + tx.Fee)
    s.IncrementNonce(tx.From)
    
    // Apply type-specific logic
    switch tx.Type {
    case "transfer":
        s.AddBalance(tx.To, tx.Value)
    case "register_validator":
        s.RegisterValidator(tx.From, tx.Value)
    case "delegate":
        s.AddDelegation(tx.To, tx.From, tx.Value)
    case "participation":
        s.SetParticipation(tx.From, true)
    }
    
    return nil
}
```

## P2P Networking

### Network Topology

- **Transport**: TCP/IP with libp2p
- **Discovery**: Kademlia DHT for peer discovery
- **Messaging**: GossipSub for pub/sub messaging
- **Topics**: Transactions, blocks, approvals, validator registrations

### Message Types

#### 1. Transaction Message
```go
type TransactionMessage struct {
    Transaction *Transaction `json:"transaction"`
    Timestamp   int64       `json:"timestamp"`
}
```

#### 2. Block Proposal Message
```go
type BlockProposalMessage struct {
    Block       *Block      `json:"block"`
    Proposer    Address     `json:"proposer"`
    Timestamp   int64       `json:"timestamp"`
}
```

#### 3. Block Approval Message
```go
type BlockApprovalMessage struct {
    BlockHash   Hash        `json:"block_hash"`
    Approver    Address     `json:"approver"`
    Signature   []byte      `json:"signature"` // ASN.1-encoded ECDSA signature
    Timestamp   int64       `json:"timestamp"`
}
```

#### 4. Validator Registration Message
```go
type ValidatorRegistrationMessage struct {
    Validator   *Validator  `json:"validator"`
    Timestamp   int64       `json:"timestamp"`
}
```

## Storage Layer

### BoltDB Schema

```go
// Buckets
const (
    BlocksBucket      = "blocks"
    StateBucket       = "state"
    ValidatorsBucket  = "validators"
    TransactionsBucket = "transactions"
)

// Key patterns
// blocks/{height} -> Block
// state/{address} -> Account
// validators/{address} -> Validator
// transactions/{hash} -> Transaction
```

### Storage Interface

```go
type Storage interface {
    Get(key []byte) ([]byte, error)
    Put(key, value []byte) error
    Delete(key []byte) error
    Batch() Batch
    Close() error
}
```

## Security Considerations

### Cryptographic Primitives

- **Key Generation**: Secp256k1 for transaction and block signing (Ethereum-compatible)
- **Address Derivation**: Whirlpool → RIPEMD-160 → BECH-32 encoding
- **Block Hashing**: SHA-256 for block integrity
- **Merkle Proofs**: SHA-256 for state verification
- **Signature Format**: ASN.1-encoded ECDSA signatures

### Address Derivation Pipeline

```go
// Address derivation process
func pubKeyToAddress(pubKey *btcec.PublicKey) Address {
    // 1. Serialize public key (compressed format)
    pubKeyBytes := pubKey.SerializeCompressed()
    
    // 2. Whirlpool hash (32 bytes)
    whirlpoolHash := whirlpool.Sum(pubKeyBytes)
    
    // 3. RIPEMD-160 hash (20 bytes)
    ripemdHash := ripemd160.Sum(whirlpoolHash[:])
    
    // 4. Return as 20-byte address
    return Address(ripemdHash)
}

// BECH-32 encoding for display
func addressToBech32(addr Address) string {
    return bech32.Encode("dyphira", addr[:])
}
```

### Attack Vectors and Mitigations

1. **Sybil Attacks**: Mitigated by stake-based validator selection
2. **Double Spending**: Prevented by nonce validation and state consistency
3. **Network Partitioning**: Handled by committee-based consensus
4. **Validator Collusion**: Limited by committee rotation and stake requirements
5. **Cryptographic Attacks**: Mitigated by using well-established Secp256k1 curve
6. **Address Collision**: Extremely unlikely due to 160-bit address space

## Performance Characteristics

### Benchmarks

Based on testing with 2-6 nodes:

- **Block Production**: ~2 seconds per block
- **Transaction Throughput**: 6-10% inclusion rate
- **Committee Formation**: 1-2 validators per committee
- **Network Latency**: Sub-second message propagation
- **State Updates**: Immediate for included transactions
- **Cryptographic Operations**: Efficient Secp256k1 signing and verification

### Scalability Considerations

- **Horizontal Scaling**: Add more validators to increase throughput
- **Vertical Scaling**: Optimize block size and transaction processing
- **Network Scaling**: Use DHT for efficient peer discovery
- **Storage Scaling**: BoltDB provides ACID transactions and efficient storage
- **Cryptographic Scaling**: Secp256k1 operations are highly optimized

## Testing Framework

### Test Categories

1. **Unit Tests**: Individual component testing
2. **Integration Tests**: Multi-component interaction testing
3. **Network Tests**: Multi-node network simulation
4. **Stress Tests**: High-load performance testing
5. **Resilience Tests**: Fault tolerance and recovery testing
6. **Cryptographic Tests**: Key generation, signing, verification, and address derivation

### Test Scenarios

```bash
# Basic functionality
./run_network.sh test

# Multi-node testing
./run_network.sh test-multi

# Complex scenarios
./run_network.sh test-scenarios

# Stress testing
./run_network.sh test-stress

# Resilience testing
./run_network.sh test-resilience
```

### Analysis Tools

```bash
# Comprehensive analysis
./analyze_results.sh
```

Provides metrics for:
- Committee formation analysis
- Transaction processing statistics
- Validator registration metrics
- Block production analysis
- Network performance summary
- Cryptographic operation performance

## Configuration

### Node Configuration

```go
type Config struct {
    Port            int    `json:"port"`
    PeerAddress     string `json:"peer_address"`
    DatabasePath    string `json:"database_path"`
    EpochLength     int    `json:"epoch_length"`
    CommitteeSize   int    `json:"committee_size"`
    BlockInterval   int    `json:"block_interval"`
    BatchSize       int    `json:"batch_size"`           // Transaction batch size
    BatchTimeout    int    `json:"batch_timeout"`        // Batch flush timeout (ms)
    MetricsInterval int    `json:"metrics_interval"`      // Metrics collection interval (ms)
    FastSync        bool   `json:"fast_sync"`             // Enable fast sync
    GracefulShutdown bool `json:"graceful_shutdown"`      // Enable graceful shutdown
}
```

### Network Configuration

- **Default Port**: 8080
- **Epoch Length**: 10 blocks
- **Committee Size**: Dynamic (1-2 for small networks)
- **Block Interval**: 2 seconds
- **Approval Timeout**: 10 seconds
- **Address Prefix**: "dyphira" for BECH-32 encoding

## Future Enhancements

### Planned Features

1. **Smart Contracts**: EVM-compatible smart contract support
2. **Cross-Chain Bridges**: Interoperability with other blockchains
3. **Advanced Consensus**: BFT consensus for better finality
4. **API Layer**: REST and gRPC APIs for external integration
5. **Monitoring**: Prometheus metrics and Grafana dashboards

### Scalability Improvements

1. **Sharding**: Horizontal scaling through state sharding
2. **Layer 2**: Rollup solutions for higher throughput
3. **Optimizations**: Parallel transaction processing
4. **Caching**: Redis-based caching for frequently accessed data

## Conclusion

Dyphira L1 provides a robust, scalable, and secure DPoS blockchain implementation with comprehensive testing and monitoring capabilities. The modular architecture allows for easy extension and customization while maintaining high performance and reliability. The cryptographic implementation using Secp256k1 ensures compatibility with Ethereum-based systems while providing strong security guarantees.

## Performance and Monitoring (updated)

- **Batching**: High-throughput block production with configurable batch size/timeout.
- **Fast Sync**: Rapid node synchronization, minimal downtime for new nodes.
- **Metrics**: JSON export for monitoring, Prometheus/Grafana integration possible.
- **Graceful Shutdown**: Clean shutdown, status reporting, and error handling. 