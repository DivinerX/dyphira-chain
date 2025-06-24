package main

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	crypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	core_routing "github.com/libp2p/go-libp2p/core/routing"
	discovery_routing "github.com/libp2p/go-libp2p/p2p/discovery/routing"
	"github.com/libp2p/go-libp2p/p2p/discovery/util"
	"github.com/multiformats/go-multiaddr"
)

// P2PNode represents a single node in the P2P network.
type P2PNode struct {
	host       host.Host
	dht        *dht.IpfsDHT
	pubsub     *pubsub.PubSub
	txTopic    *pubsub.Topic
	blockTopic *pubsub.Topic

	// New fields for multi-topic handling
	topics   map[string]*pubsub.Topic
	handlers map[string]func(*pubsub.Message)
	ctx      context.Context
}

// NewP2PNode creates and starts a new P2P node.
func NewP2PNode(ctx context.Context, listenPort int, privKey crypto.PrivKey) (*P2PNode, error) {
	sourceMultiAddr, err := multiaddr.NewMultiaddr(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort))
	if err != nil {
		return nil, err
	}

	var kdht *dht.IpfsDHT
	host, err := libp2p.New(
		libp2p.ListenAddrs(sourceMultiAddr),
		libp2p.Identity(privKey),
		libp2p.Routing(func(h host.Host) (core_routing.PeerRouting, error) {
			var err error
			kdht, err = dht.New(ctx, h, dht.Mode(dht.ModeServer))
			return kdht, err
		}),
		libp2p.EnableNATService(),
	)
	if err != nil {
		return nil, err
	}

	ps, err := pubsub.NewGossipSub(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("failed to create pubsub service: %w", err)
	}

	fmt.Printf("Node started with ID: %s\n", host.ID())
	for _, addr := range host.Addrs() {
		fmt.Printf("Listening on: %s/p2p/%s\n", addr, host.ID())
	}

	return &P2PNode{
		host:     host,
		dht:      kdht,
		pubsub:   ps,
		topics:   make(map[string]*pubsub.Topic),
		handlers: make(map[string]func(*pubsub.Message)),
		ctx:      ctx,
	}, nil
}

// RegisterTopic joins a topic and stores it for later use.
func (n *P2PNode) RegisterTopic(topicName string) {
	topic, err := n.pubsub.Join(topicName)
	if err != nil {
		log.Fatalf("Failed to join topic %s: %v", topicName, err)
	}
	n.topics[topicName] = topic
	log.Printf("DEBUG: Registered topic %s", topicName)
}

// Subscribe creates a single subscription that handles messages from all registered topics.
func (n *P2PNode) Subscribe(ctx context.Context, handler func(topic string, msg *pubsub.Message)) {
	for topicName, topic := range n.topics {
		sub, err := topic.Subscribe()
		if err != nil {
			log.Printf("Failed to subscribe to topic %s: %v", topicName, err)
			continue
		}
		log.Printf("DEBUG: Subscribed to topic %s", topicName)
		go n.handleSubscription(ctx, topicName, sub, handler)
	}
}

// handleSubscription is a helper to read messages from a single subscription.
func (n *P2PNode) handleSubscription(ctx context.Context, topicName string, sub *pubsub.Subscription, handler func(topic string, msg *pubsub.Message)) {
	defer sub.Cancel()
	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			log.Printf("Error reading from topic %s: %v", topicName, err)
			return
		}
		// Don't process messages from self
		if msg.ReceivedFrom == n.host.ID() {
			log.Printf("DEBUG: Ignoring message from self on topic %s", topicName)
			continue
		}
		log.Printf("DEBUG: Received message on topic %s from peer %s, size: %d bytes", topicName, msg.ReceivedFrom, len(msg.Data))
		handler(topicName, msg)
	}
}

// Publish sends a message to a specific topic.
func (n *P2PNode) Publish(ctx context.Context, topicName string, data []byte) error {
	topic, ok := n.topics[topicName]
	if !ok {
		return fmt.Errorf("not subscribed to topic: %s", topicName)
	}

	log.Printf("DEBUG: Publishing message to topic %s, size: %d bytes", topicName, len(data))
	err := topic.Publish(ctx, data)
	if err != nil {
		log.Printf("ERROR: Failed to publish to topic %s: %v", topicName, err)
		return err
	}
	log.Printf("DEBUG: Successfully published message to topic %s", topicName)
	return nil
}

// Connect establishes a connection with a peer.
func (n *P2PNode) Connect(ctx context.Context, peerAddr string) error {
	maddr, err := multiaddr.NewMultiaddr(peerAddr)
	if err != nil {
		return fmt.Errorf("failed to parse multiaddr: %w", err)
	}

	peerInfo, err := peer.AddrInfoFromP2pAddr(maddr)
	if err != nil {
		return fmt.Errorf("failed to get peer info from maddr: %w", err)
	}

	if err := n.host.Connect(ctx, *peerInfo); err != nil {
		return fmt.Errorf("failed to connect to peer: %w", err)
	}

	fmt.Printf("Successfully connected to peer: %s\n", peerInfo.ID)
	return nil
}

// Discover uses the DHT to find and connect to peers in the network.
func (n *P2PNode) Discover(ctx context.Context) {
	rendezvousString := "dyphira-l1-network"

	routingDiscovery := discovery_routing.NewRoutingDiscovery(n.dht)
	util.Advertise(ctx, routingDiscovery, rendezvousString)
	fmt.Println("Successfully announced!")

	fmt.Println("Searching for other peers...")
	peerChan, err := routingDiscovery.FindPeers(ctx, rendezvousString)
	if err != nil {
		panic(err)
	}

	var wg sync.WaitGroup
	for p := range peerChan {
		if p.ID == n.host.ID() {
			continue
		}
		wg.Add(1)
		go func(p peer.AddrInfo) {
			defer wg.Done()
			if n.host.Network().Connectedness(p.ID) != network.Connected {
				fmt.Printf("Found peer: %s, connecting...\n", p.ID)
				if err := n.host.Connect(ctx, p); err != nil {
					fmt.Printf("Failed to connect to peer %s: %s\n", p.ID, err)
				} else {
					fmt.Printf("Connected to peer %s\n", p.ID)
				}
			}
		}(p)
	}
	wg.Wait()
	fmt.Println("Peer discovery complete.")
}
