package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"

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
	dbPath := fmt.Sprintf("dyphria-%d.db", *port)
	os.Remove(dbPath) // Clean up previous DB for a fresh start

	// --- 2. Create Node Identity ---
	// Generate ECDSA private key for blockchain signing
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
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

	if err := node.Start(); err != nil {
		log.Fatalf("Failed to start node: %v", err)
	}

	// --- 4. Connect to an initial peer if provided ---
	if *peerAddr != "" {
		if err := node.p2p.Connect(ctx, *peerAddr); err != nil {
			log.Printf("Failed to connect to initial peer: %v", err)
		}
	}

	fmt.Println("Node is running. Press Ctrl+C to exit.")
	select {} // Block forever
}
