package main

import (
	"context"
	"testing"
	"time"
)

type stubNode struct{}

func TestFastSyncManager_SyncLoop(t *testing.T) {
	node := &AppNode{} // Use a real or stub node as needed
	fsm := NewFastSyncManager(node)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fsm.Start(ctx)

	// Wait for sync to complete
	time.Sleep(4 * time.Second)

	fsm.mu.Lock()
	defer fsm.mu.Unlock()
	if fsm.isSyncing {
		t.Errorf("FastSyncManager should not be syncing after syncLoop completes")
	}
}

func TestStateSnapshotExportImport(t *testing.T) {
	// Create a new state
	state := NewState()

	// Add some test accounts
	accounts := []*Account{
		{Address: Address{0x01}, Balance: 1000, Nonce: 1},
		{Address: Address{0x02}, Balance: 2000, Nonce: 2},
		{Address: Address{0x03}, Balance: 3000, Nonce: 3},
	}

	for _, acc := range accounts {
		if err := state.PutAccount(acc); err != nil {
			t.Fatalf("Failed to put account: %v", err)
		}
	}

	// Export snapshot
	exportedAccounts, err := state.ExportSnapshot()
	if err != nil {
		t.Fatalf("Failed to export snapshot: %v", err)
	}

	if len(exportedAccounts) != 3 {
		t.Fatalf("Expected 3 accounts, got %d", len(exportedAccounts))
	}

	// Create a new state for import
	newState := NewState()

	// Import snapshot
	if err := newState.ImportSnapshot(exportedAccounts); err != nil {
		t.Fatalf("Failed to import snapshot: %v", err)
	}

	// Verify imported accounts
	for _, expectedAcc := range accounts {
		importedAcc, err := newState.GetAccount(expectedAcc.Address)
		if err != nil {
			t.Fatalf("Failed to get imported account: %v", err)
		}
		if importedAcc.Balance != expectedAcc.Balance {
			t.Fatalf("Balance mismatch: expected %d, got %d", expectedAcc.Balance, importedAcc.Balance)
		}
		if importedAcc.Nonce != expectedAcc.Nonce {
			t.Fatalf("Nonce mismatch: expected %d, got %d", expectedAcc.Nonce, importedAcc.Nonce)
		}
	}

	t.Logf("Successfully exported and imported %d accounts", len(exportedAccounts))
}
