#!/bin/bash
set -e

# Clean up from previous runs
pkill -f dyphira-l1 || true
rm -f dyphira-*.db* logs/node_*.log pids/node_*.pid
mkdir -p logs pids

echo "[INFO] Building binary..."
go build -o dyphira-l1 .

# Start 4 nodes
for i in 0 1 2 3; do
    port=$((9000 + i))
    api_port=$((8081 + i))
    if [ $i -eq 0 ]; then
        ./dyphira-l1 -port $port -api-port $api_port > logs/node_${port}.log 2>&1 &
    else
        # Wait for bootstrap node to print its peer ID
        while ! grep -q "Node started with ID" logs/node_9000.log; do sleep 1; done
        peer_id=$(grep "Node started with ID" logs/node_9000.log | head -1 | awk '{print $NF}')
        ./dyphira-l1 -port $port -api-port $api_port -peer "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" > logs/node_${port}.log 2>&1 &
    fi
    echo $! > pids/node_${port}.pid
    echo "[INFO] Started node $i on P2P port $port, API port $api_port"
    sleep 2
done

echo "[INFO] Waiting for all nodes to start..."
sleep 10

# Get addresses for each node
declare -A ADDR
for i in 0 1 2 3; do
    api_port=$((8081 + i))
    ADDR[$i]=$(curl -s "http://localhost:$api_port/api/v1/status" | jq -r '.data.node_id')
    echo "[INFO] Node $i address: ${ADDR[$i]}"
done

# Check initial balances
for i in 0 1 2 3; do
    api_port=$((8081 + i))
    bal=$(curl -s "http://localhost:$api_port/api/v1/status" | jq -r '.data.stake')
    echo "[INFO] Node $i initial stake: $bal"
done

# Send real transactions: node 0 -> node 1, node 1 -> node 2, node 2 -> node 3
for i in 0 1 2; do
    from_api=$((8081 + i))
    to_addr=${ADDR[$((i+1))]}
    echo "[TEST] Node $i sending 10 tokens to Node $((i+1)) ($to_addr)"
    tx_data="{\"to\":\"$to_addr\",\"value\":10,\"fee\":1}"
    resp=$(curl -s -X POST -H "Content-Type: application/json" -d "$tx_data" "http://localhost:$from_api/api/v1/transactions")
    echo "$resp" | jq .
    if ! echo "$resp" | jq -e '.success' >/dev/null; then
        echo "[FAIL] Transaction from node $i to $((i+1)) failed"
        exit 1
    fi
done

echo "[INFO] Waiting for transactions to be included in blocks..."
sleep 20

# Check balances after transactions
for i in 0 1 2 3; do
    api_port=$((8081 + i))
    bal=$(curl -s "http://localhost:$api_port/api/v1/status" | jq -r '.data.stake')
    echo "[INFO] Node $i post-tx stake: $bal"
done

# Check block heights and consistency
heights=()
for i in 0 1 2 3; do
    api_port=$((8081 + i))
    h=$(curl -s "http://localhost:$api_port/api/v1/status" | jq -r '.data.blockchain_height')
    heights+=($h)
    echo "[INFO] Node $i height: $h"
done

if [[ "${heights[0]}" == "${heights[1]}" && "${heights[1]}" == "${heights[2]}" && "${heights[2]}" == "${heights[3]}" ]]; then
    echo "[PASS] All nodes have consistent blockchain height: ${heights[0]}"
else
    echo "[FAIL] Blockchain heights are inconsistent: ${heights[*]}"
    exit 1
fi

echo "[INFO] Multi-node transaction test complete. All nodes are in sync and transactions processed."
pkill -f dyphira-l1
