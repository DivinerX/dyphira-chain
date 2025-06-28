# Dyphira L1 CLI Usage Guide

## Overview

The Dyphira L1 blockchain node includes a comprehensive Command Line Interface (CLI) that allows users to interact with the blockchain, send transactions, check balances, and monitor node status.

## Getting Started

### 1. Start the Node with CLI Mode

```bash
# Start node with interactive CLI
./dyphira-l1 -cli -port 9000

# Start node with CLI demo (automated tests)
./dyphira-l1 -demo -port 9001

# Start node without CLI (normal mode)
./dyphira-l1 -port 9002
```

### 2. CLI Interface

When you start the node with `-cli` flag, you'll see:

```
=== Dyphira L1 DPoS Blockchain CLI ===
Type 'help' for available commands
Type 'exit' to quit

dyphira>
```

## Available Commands

### Account Management

#### `balance [address]`
Shows the balance for an address. If no address is provided, shows your own balance.

```bash
dyphira> balance
Address: dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s
Balance: 1000
Nonce: 0

dyphira> balance dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s
Address: dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s
Balance: 1000
Nonce: 0
```

#### `account [address]`
Shows detailed account information including validator status.

```bash
dyphira> account
Account Details:
  Address: dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s
  Balance: 1000
  Nonce: 0
  Validator Status: Active
  Stake: 100
  Delegated Stake: 0
  Compute Reputation: 0
  Participating: true
```

### Transaction Commands

#### `send <to> <amount> [fee]`
Sends a transaction to another address.

```bash
dyphira> send dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s 100 10
Transaction sent successfully!
  Hash: 1d7a0cecf7a59291e43c18875c35f5b18d8f212e7b2f072567c8838c4a9622ab
  To: dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s
  Amount: 100
  Fee: 10
  Nonce: 1
```

#### `delegate <validator> <amount> [fee]`
Delegates stake to a validator (not yet implemented).

```bash
dyphira> delegate dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s 50 10
Delegation not implemented yet
```

#### `register <stake> [fee]`
Registers as a validator (not yet implemented).

```bash
dyphira> register 100 10
Validator registration not implemented yet
```

### Blockchain Information

#### `block <height>`
Shows information about a specific block.

```bash
dyphira> block 1
Block #1
  Hash: 1d7a0cecf7a59291e43c18875c35f5b18d8f212e7b2f072567c8838c4a9622ab
  Previous Hash: 0000000000000000000000000000000000000000000000000000000000000000
  Timestamp: 1730034567
  Proposer: dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s
  Transactions: 1
  Transaction Root: 1d7a0cecf7a59291e43c18875c35f5b18d8f212e7b2f072567c8838c4a9622ab
```

#### `blocks [count]`
Shows recent blocks (default: 10).

```bash
dyphira> blocks 5
Recent blocks (1-5):
  1: 1d7a0cecf7a59291e43c18875c35f5b18d8f212e7b2f072567c8838c4a9622ab (1 txs)
```

#### `tx <hash>`
Shows transaction details (not yet implemented).

```bash
dyphira> tx 1d7a0cecf7a59291e43c18875c35f5b18d8f212e7b2f072567c8838c4a9622ab
Transaction lookup not implemented yet
```

### Network Information

#### `validators`
Shows all registered validators.

```bash
dyphira> validators
Validators (1 total):
  1: dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s
      Stake: 100
      Delegated: 0
      Reputation: 0
      Participating: true
```

#### `peers`
Shows connected peers.

```bash
dyphira> peers
Connected Peers (0):
```

#### `pool`
Shows transaction pool status.

```bash
dyphira> pool
Transaction Pool:
  Size: 0 transactions
```

### Node Status

#### `status`
Shows overall node status.

```bash
dyphira> status
Node Status:
  Address: 76df110d36fb3a747ddd7064b26e9b6ce85dcd62
  Blockchain Height: 0
  Transaction Pool Size: 0
  Validator Status: Active
  Stake: 100
  Delegated Stake: 0
```

#### `metrics`
Shows detailed node metrics.

```bash
dyphira> metrics
Node Metrics:
  Network:
    Connected Peers: 0
    Message Latency: 0.00 ms
  Consensus:
    Block Height: 0
    Committee Size: 0
    Approval Rate: 0.00%
  Storage:
    Transaction Count: 0
    Block Count: 0
    Validator Count: 0
  Performance:
    Memory Usage: 0 bytes
    Goroutine Count: 0
  Sync:
    Syncing: false
    Sync Progress: 0.00%
```

### Utility Commands

#### `help`
Shows all available commands.

```bash
dyphira> help
Available commands:
  help                    - Show this help message
  exit, quit              - Exit the CLI
  balance [address]       - Show balance for address (default: own address)
  account [address]       - Show account details (default: own address)
  send <to> <amount> [fee] - Send transaction to address
  delegate <validator> <amount> [fee] - Delegate stake to validator
  register <stake> [fee]  - Register as validator with stake amount
  block <height>          - Show block at height
  blocks [count]          - Show recent blocks (default: 10)
  tx <hash>               - Show transaction details
  pool                    - Show transaction pool status
  validators              - Show all validators
  metrics                 - Show node metrics
  status                  - Show node status
  peers                   - Show connected peers
```

#### `exit` or `quit`
Exits the CLI and shuts down the node gracefully.

```bash
dyphira> exit
Goodbye!
```

## Address Formats

The CLI supports multiple address formats:

1. **Bech32 Format**: `dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s`
2. **Hex Format**: `0x05e65bb43083371a029e69f54b4dea8c2f4ebd4b`
3. **Self Reference**: `self` (for your own address)

## Examples

### Complete Transaction Flow

```bash
# 1. Check your balance
dyphira> balance
Address: dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s
Balance: 1000
Nonce: 0

# 2. Send a transaction
dyphira> send dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s 100 10
Transaction sent successfully!
  Hash: 1d7a0cecf7a59291e43c18875c35f5b18d8f212e7b2f072567c8838c4a9622ab
  To: dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s
  Amount: 100
  Fee: 10
  Nonce: 1

# 3. Check updated balance
dyphira> balance
Address: dpos1qhn9hdpssvm35q57d865kn023sh5a02thht86s
Balance: 890
Nonce: 1

# 4. Check transaction pool
dyphira> pool
Transaction Pool:
  Size: 1 transactions

# 5. Check recent blocks
dyphira> blocks
Recent blocks (1-1):
  1: 1d7a0cecf7a59291e43c18875c35f5b18d8f212e7b2f072567c8838c4a9622ab (1 txs)
```

### Multi-Node Network

```bash
# Terminal 1: Start first node
./dyphira-l1 -cli -port 9000

# Terminal 2: Start second node and connect to first
./dyphira-l1 -cli -port 9001 -peer /ip4/127.0.0.1/tcp/9000/p2p/12D3KooW...

# Check peers on first node
dyphira> peers
Connected Peers (1):
  1: /ip4/127.0.0.1/tcp/9001/p2p/12D3KooW...
```

## Error Handling

The CLI provides clear error messages for common issues:

- **Insufficient Balance**: "Insufficient balance. Need 110, have 100"
- **Invalid Address**: "Error parsing address: invalid bech32 address"
- **Invalid Nonce**: "Error getting sender account: invalid nonce"
- **Network Issues**: "Error broadcasting transaction: failed to publish transaction"

## Tips

1. **Always check your balance** before sending transactions
2. **Use descriptive fees** to ensure your transaction gets processed quickly
3. **Monitor the transaction pool** to see pending transactions
4. **Use the metrics command** to monitor node performance
5. **Check validators** to see network participants
6. **Use `self`** instead of typing your full address

## Troubleshooting

### Common Issues

1. **"Insufficient balance"**: Make sure you have enough tokens (amount + fee)
2. **"Invalid nonce"**: Wait for previous transactions to be processed
3. **"Transaction not found"**: The transaction may not be in a block yet
4. **"No peers connected"**: Check network connectivity or start additional nodes

### Getting Help

- Use `help` command for available options
- Check node logs for detailed error information
- Ensure you're using the correct address format
- Verify network connectivity for multi-node setups

## Advanced Usage

### Scripting

You can automate CLI interactions by piping commands:

```bash
echo -e "balance\nsend dpos1... 100 10\nbalance\nexit" | ./dyphira-l1 -cli
```

### Monitoring

Use the metrics command regularly to monitor:
- Network connectivity
- Transaction throughput
- Memory usage
- Sync status

### Network Management

- Start multiple nodes on different ports
- Use the `-peer` flag to connect nodes
- Monitor peer connections with the `peers` command
- Check validator participation with the `validators` command 