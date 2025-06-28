package main

import (
	"errors"
	"testing"
)

func TestGracefulShutdown_Shutdown(t *testing.T) {
	gs := NewGracefulShutdown()
	var calledA, calledB bool

	gs.Register("componentA", func() error {
		calledA = true
		return nil
	})
	gs.Register("componentB", func() error {
		calledB = true
		return errors.New("failB")
	})

	gs.Shutdown("test reason")

	if !calledA || !calledB {
		t.Errorf("Not all shutdown hooks called: calledA=%v, calledB=%v", calledA, calledB)
	}

	status := gs.Status()
	if status.Reason != "test reason" {
		t.Errorf("Shutdown reason not set: got %q", status.Reason)
	}
	if status.Components["componentA"] != "stopped" {
		t.Errorf("componentA status wrong: %q", status.Components["componentA"])
	}
	if status.Components["componentB"] != "error: failB" {
		t.Errorf("componentB status wrong: %q", status.Components["componentB"])
	}
}
