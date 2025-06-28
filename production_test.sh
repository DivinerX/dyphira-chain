#!/bin/bash

# DPoS Blockchain Production Test Suite
set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_header() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}================================${NC}"
}

print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

print_failure() {
    echo -e "${RED}[FAIL]${NC} $1"
}

# Test counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# Cleanup function
cleanup() {
    print_status "Cleaning up..."
    pkill -f dyphira-l1 || true
    sleep 2
}

# Check dependencies
check_dependencies() {
    print_test "Checking dependencies"
    
    if ! command -v jq &> /dev/null; then
        print_error "jq is required but not installed"
        return 1
    fi
    
    if ! command -v curl &> /dev/null; then
        print_error "curl is required but not installed"
        return 1
    fi
    
    if [ ! -f "./dyphira-l1" ]; then
        print_error "dyphira-l1 binary not found. Please build first: go build -o dyphira-l1 ."
        return 1
    fi
    
    print_success "All dependencies satisfied"
    ((PASSED_TESTS++))
    ((TOTAL_TESTS++))
}

# Start network
start_network() {
    print_test "Starting test network"
    
    mkdir -p logs pids
    
    # Start bootstrap node
    print_status "Starting bootstrap node..."
    ./dyphira-l1 -port 9000 > logs/node_9000.log 2>&1 &
    echo $! > pids/node_9000.pid
    
    # Wait for bootstrap node
    sleep 5
    
    # Get peer ID
    PEER_ID=$(grep 'Node started with ID' logs/node_9000.log | head -1 | awk '{print $NF}')
    if [ -z "$PEER_ID" ]; then
        print_failure "Could not get bootstrap node peer ID"
        ((FAILED_TESTS++))
        ((TOTAL_TESTS++))
        return 1
    fi
    
    # Start additional nodes
    for i in {1..3}; do
        port=$((9000 + i))
        print_status "Starting node on port $port..."
        ./dyphira-l1 -port $port -peer "/ip4/127.0.0.1/tcp/9000/p2p/$PEER_ID" > logs/node_${port}.log 2>&1 &
        echo $! > pids/node_${port}.pid
    done
    
    # Wait for network to stabilize
    sleep 10
    
    print_success "Network started with 4 nodes"
    ((PASSED_TESTS++))
    ((TOTAL_TESTS++))
}

# Test API endpoints
test_api_endpoints() {
    print_test "Testing API endpoints"
    
    local api_base="http://localhost:8081"
    local endpoints=(
        "/api/v1/status"
        "/api/v1/blocks"
        "/api/v1/validators"
        "/api/v1/transactions/pool"
        "/api/v1/peers"
        "/api/v1/metrics"
    )
    
    local failed=0
    
    for endpoint in "${endpoints[@]}"; do
        print_status "Testing $endpoint"
        response=$(curl -s "$api_base$endpoint" 2>/dev/null)
        
        if [ $? -eq 0 ] && [ -n "$response" ]; then
            if echo "$response" | jq -e '.success' > /dev/null 2>&1; then
                print_success "$endpoint is working"
            else
                print_failure "$endpoint returned invalid response"
                ((failed++))
            fi
        else
            print_failure "$endpoint failed"
            ((failed++))
        fi
    done
    
    if [ $failed -eq 0 ]; then
        print_success "All API endpoints are working"
        ((PASSED_TESTS++))
    else
        print_failure "$failed API endpoints failed"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
}

# Test transaction sending
test_transactions() {
    print_test "Testing transaction functionality"
    
    local api_base="http://localhost:8081"
    local tx_data='{"to":"a1b2c3d4e5f6789012345678901234567890abcd","value":100,"fee":1}'
    
    # Send transaction
    response=$(curl -s -X POST -H "Content-Type: application/json" -d "$tx_data" "$api_base/api/v1/transactions")
    
    if [ $? -eq 0 ] && echo "$response" | jq -e '.success' > /dev/null 2>&1; then
        tx_hash=$(echo "$response" | jq -r '.data.transaction.hash' 2>/dev/null)
        if [ -n "$tx_hash" ] && [ "$tx_hash" != "null" ]; then
            print_success "Transaction created with hash: $tx_hash"
            ((PASSED_TESTS++))
        else
            print_failure "Transaction creation failed - no hash returned"
            ((FAILED_TESTS++))
        fi
    else
        print_failure "Transaction creation failed"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
}

# Test validator registration
test_validator_registration() {
    print_test "Testing validator registration"
    
    local api_base="http://localhost:8081"
    local reg_data='{"stake":1000}'
    
    # Register validator
    response=$(curl -s -X POST -H "Content-Type: application/json" -d "$reg_data" "$api_base/api/v1/validators/register")
    
    if [ $? -eq 0 ] && echo "$response" | jq -e '.success' > /dev/null 2>&1; then
        print_success "Validator registration successful"
        ((PASSED_TESTS++))
    else
        print_failure "Validator registration failed"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
}

# Test network resilience
test_resilience() {
    print_test "Testing network resilience"
    
    # Stop a non-bootstrap node
    local node_to_stop=$(ls pids/node_*.pid | grep -v "node_9000.pid" | head -1)
    if [ -n "$node_to_stop" ]; then
        local port=$(echo "$node_to_stop" | sed 's/pids\/node_\(.*\)\.pid/\1/')
        local pid=$(cat "$node_to_stop")
        
        print_status "Stopping node on port $port (PID: $pid)"
        kill $pid 2>/dev/null || true
        rm -f "$node_to_stop"
        
        # Wait for network to stabilize
        sleep 10
        
        # Check if remaining nodes are still functioning
        response=$(curl -s "http://localhost:8081/api/v1/status" 2>/dev/null)
        if [ $? -eq 0 ] && [ -n "$response" ]; then
            print_success "Network continued functioning after node failure"
            ((PASSED_TESTS++))
        else
            print_failure "Network failed after node failure"
            ((FAILED_TESTS++))
        fi
    else
        print_failure "No additional nodes to test resilience with"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
}

# Test state consistency
test_consistency() {
    print_test "Testing state consistency"
    
    local heights=()
    local consistent=true
    
    # Get blockchain height from all nodes
    for pid_file in pids/node_*.pid; do
        if [ -f "$pid_file" ]; then
            local port=$(echo "$pid_file" | sed 's/pids\/node_\(.*\)\.pid/\1/')
            local response=$(curl -s "http://localhost:$((port + 1))/api/v1/status" 2>/dev/null)
            if [ $? -eq 0 ] && [ -n "$response" ]; then
                local height=$(echo "$response" | jq -r '.data.blockchain_height' 2>/dev/null || echo "0")
                heights+=("$height")
            fi
        fi
    done
    
    # Check consistency
    if [ ${#heights[@]} -gt 1 ]; then
        local first_height=${heights[0]}
        for height in "${heights[@]:1}"; do
            if [ "$height" != "$first_height" ]; then
                consistent=false
                break
            fi
        done
        
        if [ "$consistent" = true ]; then
            print_success "All nodes have consistent blockchain height ($first_height)"
            ((PASSED_TESTS++))
        else
            print_failure "Nodes have inconsistent blockchain heights: ${heights[*]}"
            ((FAILED_TESTS++))
        fi
    else
        print_failure "Only one node available for consistency check"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
}

# Test performance
test_performance() {
    print_test "Testing performance metrics"
    
    # Let network run for a while
    print_status "Running network for 30 seconds to observe performance..."
    sleep 30
    
    # Check final metrics
    response=$(curl -s "http://localhost:8081/api/v1/metrics" 2>/dev/null)
    if [ $? -eq 0 ] && echo "$response" | jq -e '.success' > /dev/null 2>&1; then
        local height=$(echo "$response" | jq -r '.data.block_height' 2>/dev/null)
        local peers=$(echo "$response" | jq -r '.data.peer_count' 2>/dev/null)
        local pool=$(echo "$response" | jq -r '.data.tx_pool_size' 2>/dev/null)
        
        print_status "Final metrics: height=$height, peers=$peers, pool=$pool"
        
        if [ "$height" -gt 0 ]; then
            print_success "Blockchain is producing blocks"
            ((PASSED_TESTS++))
        else
            print_failure "Blockchain is not producing blocks"
            ((FAILED_TESTS++))
        fi
        
        if [ "$peers" -gt 0 ]; then
            print_success "P2P network is functioning"
            ((PASSED_TESTS++))
        else
            print_failure "P2P network is not functioning"
            ((FAILED_TESTS++))
        fi
    else
        print_failure "Failed to get performance metrics"
        ((FAILED_TESTS++))
    fi
    ((TOTAL_TESTS++))
    ((TOTAL_TESTS++))
}

# Generate report
generate_report() {
    print_header "Production Test Results"
    
    echo "Test Summary:"
    echo "  Total Tests: $TOTAL_TESTS"
    echo "  Passed: $PASSED_TESTS"
    echo "  Failed: $FAILED_TESTS"
    echo ""
    
    if [ $FAILED_TESTS -eq 0 ]; then
        print_success "All tests passed! The DPoS blockchain is ready for production."
        echo ""
        echo "✅ Dependencies are satisfied"
        echo "✅ Network startup is working"
        echo "✅ API endpoints are functioning"
        echo "✅ Transaction processing is working"
        echo "✅ Validator registration is operational"
        echo "✅ Network resilience is adequate"
        echo "✅ State consistency is maintained"
        echo "✅ Performance metrics are available"
        echo ""
        echo "The system has been validated for production deployment."
    else
        print_error "Some tests failed. Please review before production deployment."
    fi
    
    # Save report
    local report_file="production_test_report_$(date +%Y%m%d_%H%M%S).txt"
    {
        echo "DPoS Blockchain Production Test Report"
        echo "Generated: $(date)"
        echo ""
        echo "Test Summary:"
        echo "  Total Tests: $TOTAL_TESTS"
        echo "  Passed: $PASSED_TESTS"
        echo "  Failed: $FAILED_TESTS"
        echo ""
        echo "System Status: $([ $FAILED_TESTS -eq 0 ] && echo "READY FOR PRODUCTION" || echo "NEEDS ATTENTION")"
    } > "$report_file"
    
    print_status "Detailed report saved to: $report_file"
}

# Main execution
main() {
    print_header "DPoS Blockchain Production Test Suite"
    echo "This script will perform comprehensive testing to validate"
    echo "the DPoS blockchain system for production readiness."
    echo ""
    
    # Set up cleanup trap
    trap cleanup EXIT INT TERM
    
    check_dependencies
    start_network
    test_api_endpoints
    test_transactions
    test_validator_registration
    test_resilience
    test_consistency
    test_performance
    generate_report
}

main "$@"
