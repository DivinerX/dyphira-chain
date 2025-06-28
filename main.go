package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
)

const (
	APIPort = 8081 // API server port (different from P2P port)
)

func main() {
	// --- Command Line Flags ---
	port := flag.Int("port", 8080, "Port number for the node to listen on")
	peerAddr := flag.String("peer", "", "Address of a peer to connect to")
	apiPort := flag.Int("api-port", APIPort, "Port number for the API server")
	fastSyncPeer := flag.String("fast-sync-peer", "", "HTTP address of a peer to fast sync from (e.g. http://host:8081)")
	cliMode := flag.Bool("cli", false, "Enable interactive CLI mode")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- 1. Initialize Graceful Shutdown Manager ---
	shutdownManager := NewGracefulShutdown()

	// --- 2. Initialization ---
	dbPath := fmt.Sprintf("dyphira-%d.db", *port)

	// Clean up previous DB for a fresh start with better error handling
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: Could not remove existing database file %s: %v", dbPath, err)
		// Wait a bit to ensure any file locks are released
		time.Sleep(100 * time.Millisecond)
	}

	// --- 3. Create Node Identity ---
	// Generate ECDSA private key for blockchain signing
	privKey, err := btcec.NewPrivateKey()
	if err != nil {
		log.Fatalf("Failed to generate ECDSA private key: %v", err)
	}

	// Generate libp2p private key
	p2pPrivKey, _, err := crypto.GenerateKeyPair(crypto.Ed25519, 256)
	if err != nil {
		log.Fatalf("Failed to generate libp2p private key: %v", err)
	}

	// --- 4. Create and Start Node ---
	node, err := NewAppNode(ctx, *port, dbPath, p2pPrivKey, privKey)
	if err != nil {
		log.Fatalf("Failed to create application node: %v", err)
	}

	// --- Fast Sync Logic ---
	if *fastSyncPeer != "" && node.bc.Height() < 10 {
		log.Printf("Attempting fast sync from peer: %s", *fastSyncPeer)
		resp, err := http.Get(*fastSyncPeer + "/api/v1/state/snapshot")
		if err != nil {
			log.Fatalf("Failed to fetch snapshot from peer: %v", err)
		}
		defer resp.Body.Close()
		var apiResp struct {
			Success bool
			Data    []*Account
			Error   string
		}
		if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
			log.Fatalf("Failed to decode snapshot response: %v", err)
		}
		if !apiResp.Success {
			log.Fatalf("Peer returned error: %s", apiResp.Error)
		}
		if err := node.state.ImportSnapshot(apiResp.Data); err != nil {
			log.Fatalf("Failed to import snapshot: %v", err)
		}
		log.Printf("Fast sync: imported %d accounts from peer", len(apiResp.Data))

		// Fetch peer's latest block height
		heightResp, err := http.Get(*fastSyncPeer + "/api/v1/node/status")
		if err != nil {
			log.Fatalf("Failed to fetch peer status: %v", err)
		}
		defer heightResp.Body.Close()
		var statusResp struct {
			Success bool
			Data    map[string]interface{}
			Error   string
		}
		if err := json.NewDecoder(heightResp.Body).Decode(&statusResp); err != nil {
			log.Fatalf("Failed to decode status response: %v", err)
		}
		if !statusResp.Success {
			log.Fatalf("Peer returned error: %s", statusResp.Error)
		}
		peerHeight, ok := statusResp.Data["height"].(float64)
		if !ok {
			log.Fatalf("Peer status missing height field")
		}
		localHeight := node.bc.Height()
		for h := localHeight + 1; h <= uint64(peerHeight); h++ {
			blockResp, err := http.Get(fmt.Sprintf("%s/api/v1/blocks/%d", *fastSyncPeer, h))
			if err != nil {
				log.Fatalf("Failed to fetch block %d: %v", h, err)
			}
			defer blockResp.Body.Close()
			var blockAPIResp struct {
				Success bool
				Data    *Block
				Error   string
			}
			if err := json.NewDecoder(blockResp.Body).Decode(&blockAPIResp); err != nil {
				log.Fatalf("Failed to decode block %d: %v", h, err)
			}
			if !blockAPIResp.Success || blockAPIResp.Data == nil {
				log.Fatalf("Peer returned error for block %d: %s", h, blockAPIResp.Error)
			}
			if err := node.bc.AddBlock(blockAPIResp.Data); err != nil {
				log.Fatalf("Failed to add block %d: %v", h, err)
			}
			log.Printf("Fast sync: imported block %d", h)
		}
		log.Printf("Fast sync complete: chain height is now %d", node.bc.Height())
	}

	// --- 5. Register Node Components for Graceful Shutdown ---
	shutdownManager.Register("p2p", func() error {
		log.Printf("Shutting down P2P component...")
		return node.p2p.host.Close()
	})
	shutdownManager.Register("blockchain", func() error {
		log.Printf("Shutting down blockchain component...")
		return node.chainStore.Close()
	})
	shutdownManager.Register("validators", func() error {
		log.Printf("Shutting down validator registry...")
		return node.validatorStore.Close()
	})
	shutdownManager.Register("handshake", func() error {
		log.Printf("Shutting down handshake manager...")
		if node.handshakeManager != nil {
			// Stop handshake manager if it has a stop method
			return nil
		}
		return nil
	})
	shutdownManager.Register("optimistic-push", func() error {
		log.Printf("Shutting down optimistic push manager...")
		if node.optimisticPushManager != nil {
			// Stop optimistic push manager if it has a stop method
			return nil
		}
		return nil
	})
	shutdownManager.Register("inactivity-monitor", func() error {
		log.Printf("Shutting down inactivity monitor...")
		if node.inactivityMonitor != nil {
			node.inactivityMonitor.Stop()
		}
		return nil
	})

	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}

	// --- 6. Connect to an initial peer if provided ---
	if *peerAddr != "" {
		if err := node.p2p.Connect(ctx, *peerAddr); err != nil {
			log.Printf("Failed to connect to initial peer: %v", err)
		}
	}

	// --- 7. Start API Server ---
	apiServer := NewAPIServer(node, shutdownManager, *apiPort)
	if err := apiServer.Start(); err != nil {
		log.Fatalf("Failed to start API server: %v", err)
	}
	shutdownManager.Register("api-server", func() error {
		log.Printf("Shutting down API server...")
		// The API server will be shut down when the process exits
		return nil
	})

	// --- CLI Mode ---
	if *cliMode {
		cli := NewCLI(node, os.Stdin, os.Stdout)
		cli.Start()
		return
	}

	// --- 8. Set up graceful shutdown signal handling ---
	shutdownManager.ListenAndServe()

	fmt.Printf("Node is running on P2P port %d, API port %d. Press Ctrl+C to exit.\n", *port, *apiPort)

	// Wait for shutdown signal (handled by shutdown manager)
	<-ctx.Done()
	fmt.Println("\nShutdown initiated...")

	// Trigger graceful shutdown
	shutdownManager.Shutdown("manual shutdown")

	fmt.Println("Node shutdown complete.")
}
