# DPoS Network Testing Guide

This guide shows you how to run multiple nodes and test the DPoS (Delegated Proof of Stake) network functionality.

## Prerequisites

1. **Build the application:**
   ```bash
   go build -o dyphira-l1 .
   ```

2. **Make the testing script executable:**
   ```bash
   chmod +x run_network.sh
   ```

## Quick Start

### 1. Run a Quick Test (2 nodes)
```bash
./run_network.sh test
```
This will:
- Start 2 nodes (ports 9000 and 9001)
- Let them run for 30 seconds
- Show you the results
- Automatically clean up

### 2. Start a Single Node
```bash
./run_network.sh start
```
This starts a bootstrap node on port 9000.

### 3. Start a Multi-Node Network (4 nodes)
```bash
./run_network.sh start multi
```
This starts 4 nodes:
- Bootstrap node on port 9000
- Node 1 on port 9001 (connects to bootstrap)
- Node 2 on port 9002 (connects to bootstrap)
- Node 3 on port 9003 (connects to bootstrap)

## Manual Testing

### Step 1: Start the Bootstrap Node
```bash
./dyphira-l1 -port 9000
```

### Step 2: Get the Bootstrap Node's Peer ID
Look for this line in the output:
```
Node started with ID: 12D3KooWNXvdWHNeNvoHzjDQejH8XKZvWbCjezmM47TmkvvaGjrX
```

### Step 3: Start Additional Nodes
In new terminal windows, start nodes that connect to the bootstrap:

```bash
# Terminal 2
./dyphira-l1 -port 9001 -peer /ip4/127.0.0.1/tcp/9000/p2p/12D3KooWNXvdWHNeNvoHzjDQejH8XKZvWbCjezmM47TmkvvaGjrX

# Terminal 3
./dyphira-l1 -port 9002 -peer /ip4/127.0.0.1/tcp/9000/p2p/12D3KooWNXvdWHNeNvoHzjDQejH8XKZvWbCjezmM47TmkvvaGjrX

# Terminal 4
./dyphira-l1 -port 9003 -peer /ip4/127.0.0.1/tcp/9000/p2p/12D3KooWNXvdWHNeNvoHzjDQejH8XKZvWbCjezmM47TmkvvaGjrX
```

## What to Look For

### 1. **Node Startup**
- Each node should show: "Node started with ID: [peer-id]"
- Each node should show: "Node [address] registered as validator"

### 2. **Network Connectivity**
- Look for: "Successfully announced!" and "Peer discovery complete."
- Nodes should discover each other automatically

### 3. **Consensus Activity**
- Look for committee elections: "elected new committee for epoch"
- Look for block production: "IS the proposer. Producing block..."
- Look for block broadcasting: "broadcasting block proposal #X"
- Look for block publishing: "SUCCESS: published block proposal #X"

### 4. **Multi-Node Behavior**
With multiple nodes, you should see:
- Different nodes being elected as proposers
- Blocks being proposed by different nodes
- Consensus among the committee members

## Testing Scenarios

### Scenario 1: Single Node Operation
```bash
./run_network.sh start
./run_network.sh logs 9000
```
**Expected:** Node produces blocks every 2 seconds, always as the proposer.

### Scenario 2: Two Node Consensus
```bash
./run_network.sh test
```
**Expected:** Both nodes participate in consensus, blocks are produced by different nodes.

### Scenario 3: Four Node Network
```bash
./run_network.sh start multi
./run_network.sh status
```
**Expected:** All 4 nodes participate, proposer selection rotates among them.

### Scenario 4: Node Failure and Recovery
1. Start a 4-node network
2. Kill one node: `./run_network.sh stop`
3. Check status: `./run_network.sh status`
4. Restart: `./run_network.sh restart multi`

## Monitoring and Debugging

### Check Node Status
```bash
./run_network.sh status
```

### View Node Logs
```bash
# View logs for specific node
./run_network.sh logs 9000

# View logs in real-time
tail -f logs/node_9000.log
```

### Check Database Files
```bash
ls -la dyphira-*.db
```

## Network Management

### Stop All Nodes
```bash
./run_network.sh stop
```

### Restart Network
```bash
./run_network.sh restart multi
```

### Clean Up Everything
```bash
./run_network.sh clean
```

## Troubleshooting

### Common Issues

1. **Port Already in Use**
   ```bash
   # Check what's using the port
   lsof -i :9000
   
   # Kill the process
   pkill -f dyphira-l1
   ```

2. **Database Lock Issues**
   ```bash
   # Clean up database files
   rm -f dyphira-*.db
   ```

3. **Nodes Not Connecting**
   - Check that the peer address is correct
   - Ensure the bootstrap node is running
   - Check firewall settings

4. **No Block Production**
   - Check that nodes are registered as validators
   - Look for committee election messages
   - Verify all nodes are connected

### Debug Mode
To see more detailed logs, you can modify the application to increase log verbosity or run with additional debugging flags.

## Expected Network Behavior

### With 1 Node
- Node is always the proposer
- Blocks produced every 2 seconds
- No consensus needed (single validator)

### With 2+ Nodes
- Proposer selection rotates among validators
- Consensus requires majority approval
- Network should be more resilient
- Block production continues even if one node fails

### Performance Metrics
- Block time: ~2 seconds
- Consensus latency: <1 second
- Network discovery: <5 seconds
- Node startup: <3 seconds

## Advanced Testing

### Load Testing
Run multiple nodes and monitor:
- Block production rate
- Network latency
- Memory usage
- CPU usage

### Fault Tolerance Testing
- Kill random nodes and observe recovery
- Test network partitions
- Verify consensus continues with majority

### Stress Testing
- Run with many nodes (10+)
- Generate many transactions
- Test under high load

## Log Analysis

Key log patterns to monitor:

```bash
# Find all block proposals
grep "broadcasting block proposal" logs/node_*.log

# Find consensus messages
grep "elected new committee" logs/node_*.log

# Find validator registrations
grep "registered as validator" logs/node_*.log

# Find network connections
grep "Successfully announced" logs/node_*.log
```

This testing framework will help you verify that your DPoS implementation works correctly with multiple nodes and handles various network scenarios properly. 