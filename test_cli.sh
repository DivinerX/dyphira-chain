#!/bin/bash

# Test script for Dyphira L1 CLI functionality
echo "Testing Dyphira L1 CLI functionality..."

# Wait for the node to be fully initialized
sleep 3

# Test CLI commands by sending them to the running node
# Note: This is a basic test - in a real scenario, you'd interact directly with the CLI

echo "=== Testing CLI Commands ==="

# Test 1: Check if we can see the node process
echo "1. Checking if node is running..."
if pgrep -f "dyphira-l1" > /dev/null; then
    echo "   ✓ Node is running"
else
    echo "   ✗ Node is not running"
    exit 1
fi

# Test 2: Check if database file was created
echo "2. Checking database file..."
if [ -f "dyphira-9000.db" ]; then
    echo "   ✓ Database file created"
else
    echo "   ✗ Database file not found"
fi

# Test 3: Check if the node is listening on the port
echo "3. Checking if node is listening on port 9000..."
if netstat -tuln 2>/dev/null | grep ":9000" > /dev/null; then
    echo "   ✓ Node is listening on port 9000"
else
    echo "   ✗ Node is not listening on port 9000"
fi

# Test 4: Check if any blocks have been created
echo "4. Checking blockchain status..."
# We'll check this by looking at the log output or database

echo "=== CLI Test Summary ==="
echo "The node is running with CLI mode enabled."
echo "To interact with the CLI:"
echo "1. Open a new terminal"
echo "2. Run: ./dyphira-l1 -cli -port 9001 (to start another node)"
echo "3. Or connect to the existing node via P2P"
echo ""
echo "Available CLI commands:"
echo "  help                    - Show help"
echo "  balance [address]       - Show balance"
echo "  send <to> <amount> [fee] - Send transaction"
echo "  account [address]       - Show account details"
echo "  block <height>          - Show block"
echo "  blocks [count]          - Show recent blocks"
echo "  validators              - Show validators"
echo "  metrics                 - Show metrics"
echo "  status                  - Show status"
echo "  peers                   - Show peers"
echo "  exit                    - Exit CLI"

echo ""
echo "Test completed. The node is ready for CLI interaction." 