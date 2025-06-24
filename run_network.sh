#!/bin/bash

# DPoS Network Testing Script
# This script helps you run multiple nodes and test the network

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Function to print colored output
print_status() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}================================${NC}"
}

# Function to cleanup background processes
cleanup() {
    print_status "Cleaning up background processes..."
    pkill -f dyphira-l1 || true
    sleep 2
}

# Set up cleanup on script exit
trap cleanup EXIT

# Check if the binary exists
if [ ! -f "./dyphira-l1" ]; then
    print_error "dyphira-l1 binary not found. Please build the project first:"
    echo "go build -o dyphira-l1 ."
    exit 1
fi

# Function to start a node
start_node() {
    local port=$1
    local peer_addr=$2
    local node_name=$3
    
    print_status "Starting node $node_name on port $port"
    
    if [ -n "$peer_addr" ]; then
        print_status "Connecting to peer: $peer_addr"
        ./dyphira-l1 -port $port -peer $peer_addr > logs/node_${port}.log 2>&1 &
    else
        ./dyphira-l1 -port $port > logs/node_${port}.log 2>&1 &
    fi
    
    local pid=$!
    echo $pid > pids/node_${port}.pid
    print_status "Node $node_name started with PID $pid"
    sleep 3  # Give the node time to start
}

# Function to stop a node
stop_node() {
    local port=$1
    local pid_file="pids/node_${port}.pid"
    
    if [ -f "$pid_file" ]; then
        local pid=$(cat $pid_file)
        print_status "Stopping node on port $port (PID: $pid)"
        kill $pid 2>/dev/null || true
        rm -f $pid_file
    fi
}

# Function to show node logs
show_logs() {
    local port=$1
    local log_file="logs/node_${port}.log"
    
    if [ -f "$log_file" ]; then
        print_status "Showing logs for node on port $port:"
        echo "----------------------------------------"
        tail -f $log_file &
        local tail_pid=$!
        echo $tail_pid > pids/tail_${port}.pid
        echo "Press Ctrl+C to stop following logs"
        wait $tail_pid 2>/dev/null || true
        rm -f pids/tail_${port}.pid
    else
        print_error "Log file not found: $log_file"
    fi
}

# Function to check node status
check_status() {
    print_status "Checking node status..."
    echo "----------------------------------------"
    
    for pid_file in pids/node_*.pid; do
        if [ -f "$pid_file" ]; then
            local port=$(echo $pid_file | sed 's/pids\/node_\(.*\)\.pid/\1/')
            local pid=$(cat $pid_file)
            
            if kill -0 $pid 2>/dev/null; then
                print_status "Node on port $port: RUNNING (PID: $pid)"
            else
                print_warning "Node on port $port: STOPPED (PID: $pid)"
                rm -f $pid_file
            fi
        fi
    done
}

# Create necessary directories
mkdir -p logs pids

# Main menu
case "${1:-help}" in
    "start")
        print_header "Starting DPoS Network"
        
        # Start the first node (bootstrap node)
        start_node 9000 "" "Bootstrap"
        
        # Start additional nodes that connect to the bootstrap node
        if [ "$2" = "multi" ]; then
            print_status "Starting additional nodes..."
            start_node 9001 "/ip4/127.0.0.1/tcp/9000/p2p/$(grep 'Node started with ID' logs/node_9000.log | head -1 | awk '{print $NF}')" "Node-1"
            start_node 9002 "/ip4/127.0.0.1/tcp/9000/p2p/$(grep 'Node started with ID' logs/node_9000.log | head -1 | awk '{print $NF}')" "Node-2"
            start_node 9003 "/ip4/127.0.0.1/tcp/9000/p2p/$(grep 'Node started with ID' logs/node_9000.log | head -1 | awk '{print $NF}')" "Node-3"
        fi
        
        print_status "Network started! Use './run_network.sh status' to check status"
        print_status "Use './run_network.sh logs <port>' to view logs"
        ;;
        
    "stop")
        print_header "Stopping DPoS Network"
        cleanup
        print_status "All nodes stopped"
        ;;
        
    "restart")
        print_header "Restarting DPoS Network"
        cleanup
        sleep 2
        ./$0 start $2
        ;;
        
    "status")
        check_status
        ;;
        
    "logs")
        if [ -z "$2" ]; then
            print_error "Please specify a port number"
            echo "Usage: $0 logs <port>"
            exit 1
        fi
        show_logs $2
        ;;
        
    "test")
        print_header "Running Network Tests"
        
        # Start a simple 2-node network
        print_status "Starting test network with 2 nodes..."
        start_node 9000 "" "Test-Bootstrap"
        sleep 5
        
        # Get the bootstrap node's peer ID
        peer_id=$(grep 'Node started with ID' logs/node_9000.log | head -1 | awk '{print $NF}')
        if [ -z "$peer_id" ]; then
            print_error "Could not get bootstrap node peer ID"
            exit 1
        fi
        
        start_node 9001 "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Test-Node"
        
        print_status "Test network started. Let it run for 30 seconds..."
        sleep 30
        
        print_status "Test completed. Stopping nodes..."
        cleanup
        
        print_status "Test results:"
        echo "Check logs/node_9000.log and logs/node_9001.log for details"
        ;;
        
    "test-multi")
        print_header "Running Multi-Node Network Tests"
        
        # Start a 4-node network
        print_status "Starting test network with 4 nodes..."
        start_node 9000 "" "Test-Bootstrap"
        sleep 5
        
        # Get the bootstrap node's peer ID
        peer_id=$(grep 'Node started with ID' logs/node_9000.log | head -1 | awk '{print $NF}')
        if [ -z "$peer_id" ]; then
            print_error "Could not get bootstrap node peer ID"
            exit 1
        fi
        
        start_node 9001 "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Test-Node-1"
        start_node 9002 "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Test-Node-2"
        start_node 9003 "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Test-Node-3"
        
        print_status "Multi-node test network started. Let it run for 45 seconds..."
        sleep 45
        
        print_status "Multi-node test completed. Stopping nodes..."
        cleanup
        
        print_status "Test results:"
        echo "Check logs/node_9000.log through logs/node_9003.log for details"
        ;;
        
    "test-scenarios")
        print_header "Running Complex Scenario Tests"
        
        # Scenario 1: 5-node network with staggered startup
        print_status "Scenario 1: 5-node network with staggered startup..."
        start_node 9000 "" "Scenario-Bootstrap"
        sleep 3
        
        peer_id=$(grep 'Node started with ID' logs/node_9000.log | head -1 | awk '{print $NF}')
        if [ -z "$peer_id" ]; then
            print_error "Could not get bootstrap node peer ID"
            exit 1
        fi
        
        start_node 9001 "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Scenario-Node-1"
        sleep 2
        start_node 9002 "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Scenario-Node-2"
        sleep 2
        start_node 9003 "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Scenario-Node-3"
        sleep 2
        start_node 9004 "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Scenario-Node-4"
        
        print_status "5-node scenario network started. Let it run for 60 seconds..."
        sleep 60
        
        print_status "Scenario test completed. Stopping nodes..."
        cleanup
        
        print_status "Scenario test results:"
        echo "Check logs/node_9000.log through logs/node_9004.log for details"
        ;;
        
    "test-stress")
        print_header "Running Stress Tests"
        
        # Stress test with 6 nodes
        print_status "Starting stress test with 6 nodes..."
        start_node 9000 "" "Stress-Bootstrap"
        sleep 3
        
        peer_id=$(grep 'Node started with ID' logs/node_9000.log | head -1 | awk '{print $NF}')
        if [ -z "$peer_id" ]; then
            print_error "Could not get bootstrap node peer ID"
            exit 1
        fi
        
        # Start all nodes quickly
        for i in {1..5}; do
            port=$((9000 + i))
            start_node $port "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Stress-Node-$i"
        done
        
        print_status "Stress test network started. Let it run for 90 seconds..."
        sleep 90
        
        print_status "Stress test completed. Stopping nodes..."
        cleanup
        
        print_status "Stress test results:"
        echo "Check logs/node_9000.log through logs/node_9005.log for details"
        ;;
        
    "test-resilience")
        print_header "Running Resilience Tests"
        
        # Test network resilience with node failures
        print_status "Starting resilience test with 4 nodes..."
        start_node 9000 "" "Resilience-Bootstrap"
        sleep 3
        
        peer_id=$(grep 'Node started with ID' logs/node_9000.log | head -1 | awk '{print $NF}')
        if [ -z "$peer_id" ]; then
            print_error "Could not get bootstrap node peer ID"
            exit 1
        fi
        
        start_node 9001 "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Resilience-Node-1"
        start_node 9002 "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Resilience-Node-2"
        start_node 9003 "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Resilience-Node-3"
        
        print_status "Network started. Let it stabilize for 20 seconds..."
        sleep 20
        
        print_status "Simulating node failure: stopping node on port 9002..."
        stop_node 9002
        sleep 10
        
        print_status "Network running with failed node for 15 seconds..."
        sleep 15
        
        print_status "Restarting failed node..."
        start_node 9002 "/ip4/127.0.0.1/tcp/9000/p2p/$peer_id" "Resilience-Node-2-Restarted"
        sleep 10
        
        print_status "Network running with recovered node for 15 seconds..."
        sleep 15
        
        print_status "Resilience test completed. Stopping all nodes..."
        cleanup
        
        print_status "Resilience test results:"
        echo "Check logs/node_9000.log through logs/node_9003.log for details"
        ;;
        
    "clean")
        print_header "Cleaning Up"
        cleanup
        rm -rf logs pids dyphira-*.db
        print_status "Cleanup completed"
        ;;
        
    "help"|*)
        print_header "DPoS Network Testing Script"
        echo "Usage: $0 <command> [options]"
        echo ""
        echo "Commands:"
        echo "  start [multi]     - Start network (add 'multi' for 4 nodes)"
        echo "  stop              - Stop all nodes"
        echo "  restart [multi]   - Restart network"
        echo "  status            - Check node status"
        echo "  logs <port>       - Show logs for specific node"
        echo "  test              - Run a quick test with 2 nodes"
        echo "  test-multi        - Run a multi-node test"
        echo "  test-scenarios    - Run complex scenario tests"
        echo "  test-stress       - Run stress tests"
        echo "  test-resilience   - Run resilience tests"
        echo "  clean             - Clean up all files and processes"
        echo "  help              - Show this help"
        echo ""
        echo "Examples:"
        echo "  $0 start          # Start single bootstrap node"
        echo "  $0 start multi    # Start 4-node network"
        echo "  $0 logs 9000      # Show logs for node on port 9000"
        echo "  $0 test           # Run quick test"
        echo "  $0 test-multi     # Run multi-node test"
        echo "  $0 test-scenarios # Run complex scenario tests"
        echo "  $0 test-stress    # Run stress tests"
        echo "  $0 test-resilience # Run resilience tests"
        ;;
esac 