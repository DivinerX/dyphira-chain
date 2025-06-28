# Dyphira L1 REST API Documentation

## Overview

The Dyphira L1 blockchain provides a comprehensive REST API for interacting with the network. The API is designed to be RESTful and follows standard HTTP conventions.

## Base URL

```
http://localhost:8081/api/v1
```

## Authentication

Currently, the API does not require authentication. All endpoints are publicly accessible.

## Response Format

All API responses follow a consistent format:

```json
{
  "success": true,
  "data": { ... },
  "message": "Optional message"
}
```

## Endpoints

### Health Check

**GET** `/health`

Returns the health status of the node.

**Response:**
```json
{
  "success": true,
  "data": {
    "node_id": "76dd392ab9565a85cf1485d6c5937d979c580a9b",
    "status": "healthy",
    "timestamp": 1751051610
  }
}
```

### Node Status

**GET** `/api/v1/status`

Returns comprehensive node status information.

**Response:**
```json
{
  "success": true,
  "data": {
    "node_id": "76dd392ab9565a85cf1485d6c5937d979c580a9b",
    "blockchain_height": 1,
    "is_validator": true,
    "participating": true,
    "stake": 100,
    "delegated_stake": 0,
    "connected_peers": 0,
    "transaction_pool": 0
  }
}
```

### Network Information

**GET** `/api/v1/network`

Returns network-wide statistics and information.

**Response:**
```json
{
  "success": true,
  "data": {
    "connected_peers": 0,
    "total_validators": 1,
    "active_validators": 1,
    "current_height": 0,
    "transaction_pool": 0,
    "average_block_time": 0,
    "transaction_tps": 0
  }
}
```

### Validators

**GET** `/api/v1/validators`

Returns a list of all validators in the network.

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "address": "76dd392ab9565a85cf1485d6c5937d979c580a9b",
      "stake": 100,
      "delegated_stake": 0,
      "compute_reputation": 0,
      "participating": true
    }
  ]
}
```

### Blocks

**GET** `/api/v1/blocks`

Returns a list of recent blocks.

**Query Parameters:**
- `limit` (optional): Number of blocks to return (default: 10, max: 100)

**Response:**
```json
{
  "success": true,
  "data": [
    {
      "height": 1,
      "hash": "ba365a88617f461edfc4631089d6305700b9bbfde68e45fa4b6baa929bd4c14e",
      "previous_hash": "ab6718fae9728802f355432d08a5581026d82fd81b2ac3943d92832253ad512e",
      "proposer": "76dd392ab9565a85cf1485d6c5937d979c580a9b",
      "timestamp": 1751051609410273904,
      "size": 552,
      "transactions": 0
    }
  ]
}
```

### Block by Height

**GET** `/api/v1/blocks/{height}`

Returns a specific block by its height.

**Response:**
```json
{
  "success": true,
  "data": {
    "height": 1,
    "hash": "ba365a88617f461edfc4631089d6305700b9bbfde68e45fa4b6baa929bd4c14e",
    "previous_hash": "ab6718fae9728802f355432d08a5581026d82fd81b2ac3943d92832253ad512e",
    "proposer": "76dd392ab9565a85cf1485d6c5937d979c580a9b",
    "timestamp": 1751051609410273904,
    "size": 552,
    "transactions": []
  }
}
```

### Transaction Pool

**GET** `/api/v1/transactions/pool`

Returns information about the transaction pool.

**Response:**
```json
{
  "success": true,
  "data": {
    "size": 0
  }
}
```

### Create Transaction

**POST** `/api/v1/transactions`

Creates and broadcasts a new transaction.

**Request Body:**
```json
{
  "to": "recipient_address",
  "value": 50,
  "fee": 10,
  "type": "transfer"
}
```

**Response:**
```json
{
  "success": true,
  "data": {
    "hash": "7c4ab73bd04e0fd3c270af3728dd50f31361224b0dab1c17fb0b5516a9008076",
    "from": "76dd392ab9565a85cf1485d6c5937d979c580a9b",
    "to": "746573745f726563697069656e745f3132330000",
    "value": 50,
    "fee": 10,
    "nonce": 1,
    "type": "transfer",
    "timestamp": 1751051617248823129
  },
  "message": "Transaction created and broadcast successfully"
}
```

### Account Information

**GET** `/api/v1/accounts/{address}`

Returns account information for a specific address.

**Response:**
```json
{
  "success": true,
  "data": {
    "address": "76dd392ab9565a85cf1485d6c5937d979c580a9b",
    "balance": 1000,
    "nonce": 0
  }
}
```

### Account Balance

**GET** `/api/v1/accounts/{address}/balance`

Returns the balance for a specific address.

**Response:**
```json
{
  "success": true,
  "data": {
    "address": "76dd392ab9565a85cf1485d6c5937d979c580a9b",
    "balance": 1000
  }
}
```

### Peers

**GET** `/api/v1/peers`

Returns a list of connected peers.

**Response:**
```json
{
  "success": true,
  "data": [
    "12D3KooWDXKyYERsqDuNFxVjVD7xHvYMhm4dh4Ea1h5iPvW6KKcy"
  ]
}
```

### Metrics

**GET** `/api/v1/metrics`

Returns detailed node metrics.

**Response:**
```json
{
  "success": true,
  "data": {
    "timestamp": "2025-06-27T14:13:15Z",
    "node_id": "76dd392ab9565a85cf1485d6c5937d979c580a9b",
    "network": {
      "peer_count": 0,
      "message_latency": 0,
      "bandwidth_in": 0,
      "bandwidth_out": 0,
      "connection_errors": 0
    },
    "consensus": {
      "block_height": 1,
      "block_time": 0,
      "committee_size": 1,
      "approval_rate": 0,
      "fork_count": 0,
      "finality_time": 0
    },
    "storage": {
      "database_size_bytes": 0,
      "transaction_count": 0,
      "block_count": 1,
      "validator_count": 1
    },
    "performance": {
      "memory_usage_bytes": 0,
      "cpu_usage_percent": 0,
      "goroutine_count": 0,
      "transaction_tps": 0,
      "block_tps": 0
    },
    "sync": {
      "syncing": false,
      "sync_progress": 0,
      "sync_speed": 0,
      "peers_syncing": 0,
      "sync_errors": 0
    }
  }
}
```

## Error Responses

When an error occurs, the API returns an error response:

```json
{
  "success": false,
  "error": "Error message",
  "code": 400
}
```

Common HTTP status codes:
- `200`: Success
- `400`: Bad Request
- `404`: Not Found
- `500`: Internal Server Error

## Usage Examples

### Using curl

```bash
# Get node status
curl http://localhost:8081/api/v1/status

# Create a transaction
curl -X POST -H "Content-Type: application/json" \
  -d '{"to":"recipient_address","value":100,"fee":5,"type":"transfer"}' \
  http://localhost:8081/api/v1/transactions

# Get account balance
curl http://localhost:8081/api/v1/accounts/76dd392ab9565a85cf1485d6c5937d979c580a9b/balance
```

### Using JavaScript

```javascript
// Get node status
const response = await fetch('http://localhost:8081/api/v1/status');
const data = await response.json();
console.log(data.data);

// Create a transaction
const txResponse = await fetch('http://localhost:8081/api/v1/transactions', {
  method: 'POST',
  headers: {
    'Content-Type': 'application/json',
  },
  body: JSON.stringify({
    to: 'recipient_address',
    value: 100,
    fee: 5,
    type: 'transfer'
  })
});
const txData = await txResponse.json();
console.log(txData.data.hash);
```

## Running the API Server

To start the Dyphira L1 node with the REST API server:

```bash
./dyphira-l1 -api-port 8081 -port 9000
```

This will start:
- The blockchain node on port 9000
- The REST API server on port 8081

## Transaction Types

The API supports the following transaction types:

- `transfer`: Standard value transfer between accounts
- `participation`: Participation in the network
- `register_validator`: Register as a validator
- `delegate`: Delegate stake to a validator

## Rate Limiting

Currently, there are no rate limits implemented. However, it's recommended to implement appropriate rate limiting for production use.

## Security Considerations

- The API is currently unauthenticated and should not be exposed to the public internet without proper security measures
- Consider implementing authentication and authorization for production deployments
- Use HTTPS in production environments
- Implement rate limiting to prevent abuse 