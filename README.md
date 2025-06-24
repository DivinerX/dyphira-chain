# Dyphira L1 DPoS Blockchain - Developer Documentation

## Overview

This project implements a Delegated Proof-of-Stake (DPoS) blockchain node in Go, featuring:

- **P2P networking** using libp2p (GossipSub, DHT, peer discovery)
- **Blockchain** with block/transaction structures, Merkle Trie state, and persistent storage (BoltDB)
- **DPoS consensus**: committee selection, proposer rotation, block approval by signatures
- **Transaction pool** and state management with complex transaction types
- **Validator registry** and committee management with delegation support
- **Comprehensive testing** with multi-node scenarios and resilience testing
- **Network analysis** and monitoring tools

---

## Project Structure

- `main.go` - Entry point, node startup, CLI flags
- `node.go` - Main node logic, consensus, networking, block/tx handling
- `p2p.go` - P2P networking (libp2p, pubsub, peer discovery)
- `blockchain.go` - Blockchain data structure, block creation, storage
- `types.go` - Core types: Block, Transaction, Validator, etc.
- `block_approval.go` - Block approval logic (signatures, threshold)
- `committee.go` - Committee and proposer selection for DPoS
- `validator_registry.go` - Validator registration and management
- `transaction_pool.go` - Transaction pool logic
- `state.go` - Account state, Merkle Trie, state transitions
- `merkle_trie.go` - Merkle Trie implementation for state
- `storage.go` - Storage abstraction (in-memory and BoltDB)
- `run_network.sh` - Network testing and management script
- `analyze_results.sh` - Test results analysis and reporting
- `*_test.go` - Unit and integration tests

---

## Dependencies

- Go 1.24+
- [libp2p](https://github.com/libp2p/go-libp2p) for P2P networking
- [bbolt](https://github.com/etcd-io/bbolt) for persistent storage
- [testify](https://github.com/stretchr/testify) for testing

See `go.mod` for the full list.

---

## Running a Node

### Build

```bash
go build -o dyphira-l1
```

### Run

```bash
./dyphira-l1 -port 9000
```

- `-port` (default: 8080): TCP port for P2P networking
- `-peer`: (optional) Multiaddress of a peer to connect to (e.g., `/ip4/127.0.0.1/tcp/9001/p2p/<peer-id>`)

Each node creates its own database file (`dyphira-<port>.db`). On startup, the node:

1. Generates ECDSA and libp2p keys
2. Registers itself as a validator with participation enabled
3. Starts P2P networking and joins the DPoS network
4. Participates in committee selection, block production, and approval
5. Creates test transactions and delegation transactions

---

## Network Testing and Management

### Quick Start with Network Script

The `run_network.sh` script provides comprehensive network testing capabilities:

```bash
# Build the binary first
go build -o dyphira-l1

# Start a single bootstrap node
./run_network.sh start

# Start a 4-node network
./run_network.sh start multi

# Run different test scenarios
./run_network.sh test          # 2-node basic test
./run_network.sh test-multi    # 4-node multi test
./run_network.sh test-scenarios # 5-node complex scenarios
./run_network.sh test-stress   # 6-node stress test
./run_network.sh test-resilience # 4-node resilience test

# Check node status
./run_network.sh status

# View logs for specific node
./run_network.sh logs 9000

# Clean up
./run_network.sh clean
```

### Test Scenarios

1. **Basic Test (2 nodes)**: Validates core functionality
2. **Multi-Node Test (4 nodes)**: Tests committee formation and transaction processing
3. **Complex Scenarios (5 nodes)**: Tests staggered startup and dynamic validator joining
4. **Stress Test (6 nodes)**: Tests network performance under load
5. **Resilience Test (4 nodes)**: Tests node failure and recovery

### Analysis and Monitoring

```bash
# Analyze test results
./analyze_results.sh
```

The analysis script provides:
- Committee formation analysis
- Transaction processing statistics
- Validator registration metrics
- Block production analysis
- Overall network performance summary

---

## Network and Consensus

- **P2P**: Uses libp2p GossipSub for pub/sub messaging and Kademlia DHT for peer discovery
- **Topics**: `/dyphira/transactions/v1`, `/dyphira/blocks/v1`, `/dyphira/approvals/v1`, `/dyphira/validators/v1`
- **DPoS**: Every epoch (10 blocks), a committee is selected based on stake, delegation, and participation
- **Block Production**: Proposer is rotated round-robin within the committee
- **Block Approval**: At least 2/3 of the committee must sign a block for it to be finalized
- **Validator Participation**: Validators must explicitly participate to be eligible for committees

---

## Transaction Types

The system supports multiple transaction types:

1. **Transfer** (`transfer`): Standard token transfers between accounts
2. **Validator Registration** (`register_validator`): Register as a validator with stake
3. **Delegation** (`delegate`): Delegate tokens to validators
4. **Participation** (`participation`): Enable/disable validator participation

### Transaction Structure

```go
type Transaction struct {
    From      Address `json:"from"`
    To        Address `json:"to"`
    Value     uint64  `json:"value"`
    Nonce     uint64  `json:"nonce"`
    Fee       uint64  `json:"fee"`
    Timestamp int64   `json:"timestamp"`
    Type      string  `json:"type"`
    Hash      Hash    `json:"hash"`
    Signature []byte  `json:"signature"`
}
```

---

## Key Components

### AppNode (`node.go`)

- Orchestrates all subsystems: P2P, blockchain, state, tx pool, validator registry
- Handles network messages (transactions, block proposals, approvals, validator registrations)
- Runs the main producer loop for block creation and committee management
- Manages test transaction generation and delegation transactions
- Handles account initialization for all known validators

### P2PNode (`p2p.go`)

- Manages libp2p host, pubsub, DHT, and peer connections
- Handles topic registration, message publishing, and peer discovery
- Supports validator registration broadcasting

### Blockchain (`blockchain.go`)

- Stores blocks in a key-value store (BoltDB or in-memory)
- Handles block creation, validation, and retrieval
- Supports transaction inclusion and block finalization

### State (`state.go`, `merkle_trie.go`)

- Manages account balances and nonces using a Merkle Trie
- Applies transactions and blocks to update state
- Handles fee deduction and balance validation
- Supports delegation tracking

### ValidatorRegistry (`validator_registry.go`)

- Registers, updates, and lists validators
- Manages validator participation status
- Used for committee selection
- Supports delegation tracking

### Committee and Proposer Selection (`committee.go`)

- Selects committee members for each epoch based on stake, delegation, and participation
- Rotates proposer for block production
- Handles inactive validator replacement
- Supports dynamic committee size adjustment

### Block Approval (`block_approval.go`)

- Tracks committee signatures for each block
- Finalizes blocks when threshold is reached
- Handles approval timeout and retry logic

### Transaction Pool (`transaction_pool.go`)

- Validates and pools incoming transactions
- Selects transactions for block inclusion
- Supports multiple transaction types
- Handles nonce validation and balance checks

---

## Testing

### Unit Tests

Run all tests:

```bash
go test -v
```

### Integration Tests

The integration tests in `node_test.go` simulate:
- Multiple nodes with P2P networking
- Block production and consensus
- Transaction inclusion and processing
- Validator registration and delegation
- Committee formation and rotation
- Network resilience and recovery

### Network Testing

Use the provided scripts for comprehensive network testing:

```bash
# Run all test scenarios
./run_network.sh test
./run_network.sh test-multi
./run_network.sh test-scenarios
./run_network.sh test-stress
./run_network.sh test-resilience

# Analyze results
./analyze_results.sh
```

---

## Performance and Scalability

### Test Results Summary

Based on comprehensive testing:

- **Network Size**: Successfully tested with 2-6 nodes
- **Committee Formation**: 1-2 validators per committee (as expected for small networks)
- **Transaction Processing**: 6-10% transaction inclusion rate
- **Block Production**: 150+ blocks produced across all tests
- **Resilience**: Survives node failures and recovers successfully
- **Delegation**: Supports delegation transactions between validators

### Key Metrics

- **Validator Registration**: 5-10 unique validators per test
- **Transaction Types**: Transfer, delegation, validator registration
- **Network Latency**: Sub-second block production
- **Fault Tolerance**: Automatic recovery from node failures

---

## Extending the System

- **Consensus**: Modify `committee.go` and `block_approval.go` for new selection or approval logic
- **State**: Extend `state.go` and `merkle_trie.go` for smart contracts or new account types
- **Networking**: Add new pubsub topics or message types in `p2p.go` and `node.go`
- **APIs**: Expose RPC or REST endpoints by adding a server in `main.go` or a new file
- **Testing**: Add new test scenarios in `run_network.sh` and analysis in `analyze_results.sh`

---

## Example: Running a Local Network

### Manual Setup

1. Start multiple nodes on different ports:

   ```bash
   ./dyphira-l1 -port 9000
   ./dyphira-l1 -port 9001 -peer /ip4/127.0.0.1/tcp/9000/p2p/<peer-id>
   ./dyphira-l1 -port 9002 -peer /ip4/127.0.0.1/tcp/9000/p2p/<peer-id>
   ```

2. Nodes will discover each other, elect a committee, and start producing/approving blocks.

### Automated Testing

```bash
# Quick test with 2 nodes
./run_network.sh test

# Full network test with 4 nodes
./run_network.sh test-multi

# Complex scenario testing
./run_network.sh test-scenarios
```

---

## Troubleshooting

### Common Issues

1. **Port Already in Use**: Use different ports for each node
2. **Database Lock**: Ensure only one node per database file
3. **Network Connectivity**: Check firewall settings and peer addresses
4. **Committee Size**: Small networks will have smaller committees

### Log Analysis

```bash
# View logs for specific node
./run_network.sh logs 9000

# Analyze test results
./analyze_results.sh

# Check for errors
grep -i error logs/node_*.log
```

---

## Code Conventions

- All core types and logic are in the root package
- Use Go modules for dependency management
- Tests are colocated with implementation files
- Network testing scripts use bash with colored output
- Analysis scripts provide comprehensive metrics

---

## Contact

For questions or contributions, please open an issue or pull request. 