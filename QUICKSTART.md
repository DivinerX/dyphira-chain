# Dyphira L1 Quick Start Guide

Get up and running with Dyphira L1 DPoS blockchain in minutes!

## Prerequisites

- Go 1.24 or later
- Git
- Basic command line knowledge

## Installation

1. **Clone the repository**
   ```bash
   git clone <repository-url>
   cd DPoS
   ```

2. **Build the binary**
   ```bash
   go build -o dyphira-l1 .
   ```

3. **Make scripts executable**
   ```bash
   chmod +x run_network.sh analyze_results.sh
   ```

## Quick Start

### Option 1: Single Node (Development)

Start a single node for development and testing:

```bash
./dyphira-l1 -port 9000
```

This will:
- Start a node on port 9000
- Generate Secp256k1 private/public key pair
- Register as a validator
- Begin producing blocks
- Create test transactions

### Option 2: Multi-Node Network (Testing)

Run a comprehensive test with multiple nodes:

```bash
# Test with 2 nodes
./run_network.sh test

# Test with 4 nodes
./run_network.sh test-multi

# Test complex scenarios with 5 nodes
./run_network.sh test-scenarios
```

### Option 3: Manual Multi-Node Setup

1. **Start bootstrap node**
   ```bash
   ./dyphira-l1 -port 9000
   ```

2. **Get the peer ID** (from the logs)
   ```
   Node started with ID: 12D3KooW...
   ```

3. **Start additional nodes**
   ```bash
   ./dyphira-l1 -port 9001 -peer /ip4/127.0.0.1/tcp/9000/p2p/12D3KooW...
   ./dyphira-l1 -port 9002 -peer /ip4/127.0.0.1/tcp/9000/p2p/12D3KooW...
   ```

## What You'll See

### Node Startup
```
2025/06/23 16:30:00 INFO: Starting Dyphira L1 node on port 9000
2025/06/23 16:30:01 INFO: Generated Secp256k1 keys and address: dyphira1...
2025/06/23 16:30:02 INFO: Node started with ID 12D3KooW...
2025/06/23 16:30:03 INFO: Registered as validator with stake 100
2025/06/23 16:30:04 INFO: Starting P2P networking...
```

### Block Production
```
2025/06/23 16:30:10 INFO: Published block proposal #1
2025/06/23 16:30:12 INFO: Including 1 transactions in block #1
2025/06/23 16:30:14 INFO: Block #1 finalized with 2/3 approvals
```

### Committee Formation
```
2025/06/23 16:30:20 INFO: Elected new committee for epoch starting at height 0. Committee size: 2
2025/06/23 16:30:22 INFO: Current proposer: dyphira1...
```

## Monitoring and Analysis

### View Logs
```bash
# View logs for specific node
./run_network.sh logs 9000

# Follow logs in real-time
tail -f logs/node_9000.log
```

### Analyze Results
```bash
# After running tests, analyze the results
./analyze_results.sh
```

### Check Network Status
```bash
# Check if nodes are running
./run_network.sh status
```

## Understanding the Output

### Key Metrics to Watch

1. **Committee Size**: Should be 1-2 for small networks
2. **Block Production**: ~2 seconds per block
3. **Transaction Inclusion**: 6-10% of transactions included in blocks
4. **Validator Registration**: Nodes should register each other
5. **Address Format**: All addresses displayed as BECH-32 encoded (dyphira1...)

### Sample Analysis Output
```
================================
DPoS Network Test Results Analysis
================================
[INFO] Total nodes tested: 4
[INFO] Total blocks produced: 73
[INFO] Total transactions processed: 7
[INFO] Total delegation transactions: 4
[INFO] Average transactions per block: 0.09

✅ Validator registration and sharing working
✅ Committee formation working (sizes 1-2 as expected)
✅ Transaction processing and inclusion working
✅ Delegation transactions working
✅ Multi-node network scaling working
✅ Dynamic validator joining working
✅ Secp256k1 cryptographic operations working
```

## Common Commands

### Network Management
```bash
# Start network
./run_network.sh start [multi]

# Stop all nodes
./run_network.sh stop

# Restart network
./run_network.sh restart [multi]

# Clean up everything
./run_network.sh clean
```

### Testing
```bash
# Basic test (2 nodes, 30 seconds)
./run_network.sh test

# Multi-node test (4 nodes, 45 seconds)
./run_network.sh test-multi

# Complex scenarios (5 nodes, 60 seconds)
./run_network.sh test-scenarios

# Stress test (6 nodes, 90 seconds)
./run_network.sh test-stress

# Resilience test (4 nodes with failure/recovery)
./run_network.sh test-resilience
```

### Troubleshooting
```bash
# Check for errors
grep -i error logs/node_*.log

# Check committee formation
grep "committee size" logs/node_*.log

# Check transaction processing
grep "including.*transactions" logs/node_*.log

# Check validator registration
grep "registered validator" logs/node_*.log

# Check cryptographic operations
grep "signature" logs/node_*.log
```

## Configuration Options

### Command Line Flags
```bash
./dyphira-l1 -port 9000                    # Set port
./dyphira-l1 -peer <multiaddress>          # Connect to peer
```

### Environment Variables
```bash
export DYPHIRA_LOG_LEVEL=debug             # Set log level
export DYPHIRA_DB_PATH=./data              # Set database path
```

## Cryptographic Features

### Address Format
- **Internal**: 20-byte addresses derived from Secp256k1 public keys
- **Display**: BECH-32 encoded with "dyphira" prefix (e.g., `dyphira1...`)
- **Derivation**: Public Key → Whirlpool → RIPEMD-160 → BECH-32

### Transaction Signing
- **Algorithm**: Secp256k1 ECDSA (Ethereum-compatible)
- **Format**: ASN.1-encoded signatures
- **Verification**: Public key recovery and signature verification

### Key Management
- **Key Generation**: Secure Secp256k1 private/public key pairs
- **Storage**: Private keys stored securely for signing operations
- **Compatibility**: Fully compatible with Ethereum-based systems

## Next Steps

1. **Explore the Code**: Check out the source code in the main files
2. **Run Tests**: Try different test scenarios
3. **Modify Configuration**: Adjust parameters for your needs
4. **Add Features**: Extend the system with new transaction types
5. **Deploy**: Set up nodes on different machines

## Getting Help

- **Documentation**: See `README.md` for detailed documentation
- **Technical Specs**: See `TECHNICAL_SPEC.md` for implementation details
- **Issues**: Check the logs for error messages
- **Analysis**: Use `./analyze_results.sh` for performance insights

## Example Workflow

Here's a complete example workflow:

```bash
# 1. Build the project
go build -o dyphira-l1 .

# 2. Run a quick test
./run_network.sh test

# 3. Check the results
./analyze_results.sh

# 4. View detailed logs
./run_network.sh logs 9000

# 5. Clean up
./run_network.sh clean
```

That's it! You now have a fully functional DPoS blockchain network running locally with secure Secp256k1 cryptography. 🚀 