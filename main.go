package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
)

func main() {
	// --- Command Line Flags ---
	port := flag.Int("port", 8080, "Port number for the node to listen on")
	peerAddr := flag.String("peer", "", "Address of a peer to connect to")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- 1. Initialization ---
	dbPath := fmt.Sprintf("dyphira-%d.db", *port)

	// Clean up previous DB for a fresh start with better error handling
	if err := os.Remove(dbPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: Could not remove existing database file %s: %v", dbPath, err)
		// Wait a bit to ensure any file locks are released
		time.Sleep(100 * time.Millisecond)
	}

	// --- 2. Create Node Identity ---
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

	// --- 3. Create and Start Node ---
	node, err := NewAppNode(ctx, *port, dbPath, p2pPrivKey, privKey)
	if err != nil {
		log.Fatalf("Failed to create application node: %v", err)
	}
	defer node.Close()

	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}

	// --- 4. Connect to an initial peer if provided ---
	if *peerAddr != "" {
		if err := node.p2p.Connect(ctx, *peerAddr); err != nil {
			log.Printf("Failed to connect to initial peer: %v", err)
		}
	}

	// --- 5. Set up graceful shutdown ---
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Node is running. Press Ctrl+C to exit.")

	// Wait for shutdown signal
	<-sigChan
	fmt.Println("\nShutting down gracefully...")

	// The deferred node.Close() will be called here.

	fmt.Println("Node shutdown complete.")
}
