package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type ShutdownStatus struct {
	Reason     string
	Components map[string]string // component name -> status
}

type GracefulShutdown struct {
	ctx        context.Context
	cancel     context.CancelFunc
	components []func() error
	statusMu   sync.Mutex
	status     ShutdownStatus
}

func NewGracefulShutdown() *GracefulShutdown {
	ctx, cancel := context.WithCancel(context.Background())
	return &GracefulShutdown{
		ctx:    ctx,
		cancel: cancel,
		status: ShutdownStatus{Components: make(map[string]string)},
	}
}

// Register a shutdown hook for a component
func (gs *GracefulShutdown) Register(component string, shutdownFunc func() error) {
	gs.components = append(gs.components, func() error {
		err := shutdownFunc()
		gs.statusMu.Lock()
		if err != nil {
			gs.status.Components[component] = "error: " + err.Error()
		} else {
			gs.status.Components[component] = "stopped"
		}
		gs.statusMu.Unlock()
		return err
	})
}

// Start listening for OS signals and trigger shutdown
func (gs *GracefulShutdown) ListenAndServe() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigChan
		log.Printf("GracefulShutdown: Received signal %v, shutting down...", sig)
		gs.Shutdown("signal: " + sig.String())
	}()
}

// Shutdown all registered components in order
func (gs *GracefulShutdown) Shutdown(reason string) {
	gs.statusMu.Lock()
	gs.status.Reason = reason
	gs.statusMu.Unlock()
	gs.cancel()
	for _, shutdownFunc := range gs.components {
		_ = shutdownFunc()
	}
	log.Printf("GracefulShutdown: All components shut down. Reason: %s", reason)
}

// Status returns the current shutdown status
func (gs *GracefulShutdown) Status() ShutdownStatus {
	gs.statusMu.Lock()
	defer gs.statusMu.Unlock()
	return gs.status
}
