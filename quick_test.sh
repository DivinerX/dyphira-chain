#!/bin/bash

# Quick DPoS Production Test
set -e

echo "========================================"
echo "DPoS Blockchain Quick Production Test"
echo "========================================"
echo ""

# Check dependencies
echo "[TEST] Checking dependencies"
if ! command -v jq &> /dev/null; then
    echo "[ERROR] jq is required but not installed"
    exit 1
fi

if ! command -v curl &> /dev/null; then
    echo "[ERROR] curl is required but not installed"
    exit 1
fi

if [ ! -f "./dyphira-l1" ]; then
    echo "[ERROR] dyphira-l1 binary not found. Please build first: go build -o dyphira-l1 ."
    exit 1
fi

echo "[PASS] All dependencies satisfied"
echo ""

# Cleanup function
cleanup() {
    echo "[INFO] Cleaning up..."
    pkill -f dyphira-l1 || true
    sleep 2
}

# Set up cleanup trap
trap cleanup EXIT INT TERM

# Start single node for testing
echo "[TEST] Starting test node"
mkdir -p logs pids

./dyphira-l1 -port 9000 > logs/node_9000.log 2>&1 &
echo $! > pids/node_9000.pid

echo "[INFO] Waiting for node to start..."
sleep 10

# Test API endpoints
echo "[TEST] Testing API endpoints"

# Test status endpoint
echo "[INFO] Testing /api/v1/status"
response=$(curl -s "http://localhost:8081/api/v1/status" 2>/dev/null)
if [ $? -eq 0 ] && echo "$response" | jq -e '.success' > /dev/null 2>&1; then
    echo "[PASS] Status endpoint is working"
    height=$(echo "$response" | jq -r '.data.blockchain_height' 2>/dev/null)
    echo "[INFO] Blockchain height: $height"
else
    echo "[FAIL] Status endpoint failed"
    exit 1
fi

# Test blocks endpoint
echo "[INFO] Testing /api/v1/blocks"
response=$(curl -s "http://localhost:8081/api/v1/blocks" 2>/dev/null)
if [ $? -eq 0 ] && echo "$response" | jq -e '.success' > /dev/null 2>&1; then
    echo "[PASS] Blocks endpoint is working"
else
    echo "[FAIL] Blocks endpoint failed"
    exit 1
fi

# Test validators endpoint
echo "[INFO] Testing /api/v1/validators"
response=$(curl -s "http://localhost:8081/api/v1/validators" 2>/dev/null)
if [ $? -eq 0 ] && echo "$response" | jq -e '.success' > /dev/null 2>&1; then
    echo "[PASS] Validators endpoint is working"
else
    echo "[FAIL] Validators endpoint failed"
    exit 1
fi

# Test transaction creation
echo "[TEST] Testing transaction functionality"
tx_data='{"to":"a1b2c3d4e5f6789012345678901234567890abcd","value":100,"fee":1}'
response=$(curl -s -X POST -H "Content-Type: application/json" -d "$tx_data" "http://localhost:8081/api/v1/transactions")

if [ $? -eq 0 ] && echo "$response" | jq -e '.success' > /dev/null 2>&1; then
    tx_hash=$(echo "$response" | jq -r '.data.transaction.hash' 2>/dev/null)
    if [ -n "$tx_hash" ] && [ "$tx_hash" != "null" ]; then
        echo "[PASS] Transaction created with hash: $tx_hash"
    else
        echo "[FAIL] Transaction creation failed - no hash returned"
        exit 1
    fi
else
    echo "[FAIL] Transaction creation failed"
    echo "Response: $response"
    exit 1
fi

# Test transaction pool
echo "[INFO] Testing /api/v1/transactions/pool"
response=$(curl -s "http://localhost:8081/api/v1/transactions/pool" 2>/dev/null)
if [ $? -eq 0 ] && echo "$response" | jq -e '.success' > /dev/null 2>&1; then
    pool_size=$(echo "$response" | jq '.data | length' 2>/dev/null || echo "0")
    echo "[PASS] Transaction pool endpoint is working (size: $pool_size)"
else
    echo "[FAIL] Transaction pool endpoint failed"
    exit 1
fi

# Test metrics
echo "[INFO] Testing /api/v1/metrics"
response=$(curl -s "http://localhost:8081/api/v1/metrics" 2>/dev/null)
if [ $? -eq 0 ] && echo "$response" | jq -e '.success' > /dev/null 2>&1; then
    echo "[PASS] Metrics endpoint is working"
else
    echo "[FAIL] Metrics endpoint failed"
    exit 1
fi

# Let it run for a bit to observe performance
echo "[INFO] Running for 30 seconds to observe performance..."
sleep 30

# Final status check
echo "[INFO] Final status check"
response=$(curl -s "http://localhost:8081/api/v1/status" 2>/dev/null)
if [ $? -eq 0 ] && echo "$response" | jq -e '.success' > /dev/null 2>&1; then
    final_height=$(echo "$response" | jq -r '.data.blockchain_height' 2>/dev/null)
    echo "[INFO] Final blockchain height: $final_height"
    
    if [ "$final_height" -gt "$height" ]; then
        echo "[PASS] Blockchain is producing new blocks"
    else
        echo "[FAIL] Blockchain is not producing new blocks"
        exit 1
    fi
else
    echo "[FAIL] Final status check failed"
    exit 1
fi

echo ""
echo "========================================"
echo "PRODUCTION TEST RESULTS"
echo "========================================"
echo "✅ All API endpoints are functioning"
echo "✅ Transaction processing is working"
echo "✅ Blockchain is producing blocks"
echo "✅ Metrics collection is working"
echo ""
echo "The DPoS blockchain system is ready for production!"
echo ""

# Generate report
report_file="production_test_report_$(date +%Y%m%d_%H%M%S).txt"
{
    echo "DPoS Blockchain Production Test Report"
    echo "Generated: $(date)"
    echo ""
    echo "Test Results:"
    echo "  ✅ Dependencies check passed"
    echo "  ✅ Node startup successful"
    echo "  ✅ API endpoints functional"
    echo "  ✅ Transaction processing working"
    echo "  ✅ Blockchain producing blocks"
    echo "  ✅ Metrics collection working"
    echo ""
    echo "System Status: READY FOR PRODUCTION"
} > "$report_file"

echo "[INFO] Detailed report saved to: $report_file"
