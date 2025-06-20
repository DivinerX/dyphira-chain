# Dyphria-L1 DPoS Blockchain - Developer Documentation

## Overview

This project implements a Delegated Proof-of-Stake (DPoS) blockchain node in Go, featuring:

- **P2P networking** using libp2p (GossipSub, DHT, peer discovery)
- **Blockchain** with block/transaction structures, Merkle Trie state, and persistent storage (BoltDB)
- **DPoS consensus**: committee selection, proposer rotation, block approval by signatures
- **Transaction pool** and state management
- **Validator registry** and committee management

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
go build -o dyphria-l1
```

### Run

```bash
./dyphria-l1 -port 9000
```

- `-port` (default: 8080): TCP port for P2P networking
- `-peer`: (optional) Multiaddress of a peer to connect to (e.g., `/ip4/127.0.0.1/tcp/9001/p2p/<peer-id>`)

Each node creates its own database file (`dyphria-<port>.db`). On startup, the node:

1. Generates ECDSA and libp2p keys
2. Registers itself as a validator
3. Starts P2P networking and joins the DPoS network
4. Participates in committee selection, block production, and approval

---

## Network and Consensus

- **P2P**: Uses libp2p GossipSub for pub/sub messaging and Kademlia DHT for peer discovery.
- **Topics**: `/dyphria/transactions/v1`, `/dyphria/blocks/v1`, `/dyphria/approvals/v1`
- **DPoS**: Every epoch (10 blocks), a committee is selected based on stake, delegation, and reputation.
- **Block Production**: Proposer is rotated round-robin within the committee.
- **Block Approval**: At least 2/3 of the committee must sign a block for it to be finalized.

---

## Key Components

### AppNode (`node.go`)

- Orchestrates all subsystems: P2P, blockchain, state, tx pool, validator registry
- Handles network messages (transactions, block proposals, approvals)
- Runs the main producer loop for block creation and committee management

### P2PNode (`p2p.go`)

- Manages libp2p host, pubsub, DHT, and peer connections
- Handles topic registration, message publishing, and peer discovery

### Blockchain (`blockchain.go`)

- Stores blocks in a key-value store (BoltDB or in-memory)
- Handles block creation, validation, and retrieval

### State (`state.go`, `merkle_trie.go`)

- Manages account balances and nonces using a Merkle Trie
- Applies transactions and blocks to update state

### ValidatorRegistry (`validator_registry.go`)

- Registers, updates, and lists validators
- Used for committee selection

### Committee and Proposer Selection (`committee.go`)

- Selects committee members for each epoch based on stake, delegation, and reputation
- Rotates proposer for block production

### Block Approval (`block_approval.go`)

- Tracks committee signatures for each block
- Finalizes blocks when threshold is reached

### Transaction Pool (`transaction_pool.go`)

- Validates and pools incoming transactions
- Selects transactions for block inclusion

---

## Testing

Run all tests:

```bash
go test -v
```

Integration tests in `node_test.go` simulate multiple nodes, block production, transaction inclusion, and DPoS consensus.

---

## Extending the System

- **Consensus**: Modify `committee.go` and `block_approval.go` for new selection or approval logic.
- **State**: Extend `state.go` and `merkle_trie.go` for smart contracts or new account types.
- **Networking**: Add new pubsub topics or message types in `p2p.go` and `node.go`.
- **APIs**: Expose RPC or REST endpoints by adding a server in `main.go` or a new file.

---

## Example: Running a Local Network

1. Start multiple nodes on different ports:

   ```bash
   ./dyphria-l1 -port 9000
   ./dyphria-l1 -port 9001 -peer /ip4/127.0.0.1/tcp/9000/p2p/<peer-id>
   ./dyphria-l1 -port 9002 -peer /ip4/127.0.0.1/tcp/9000/p2p/<peer-id>
   ```

2. Nodes will discover each other, elect a committee, and start producing/approving blocks.

---

## Code Conventions

- All core types and logic are in the root package.
- Use Go modules for dependency management.
- Tests are colocated with implementation files.

---

## Contact

For questions or contributions, please open an issue or pull request. 