package main

import (
	"testing"
	"time"
)

func TestMetricsCollector(t *testing.T) {
	// Create a new metrics collector
	mc := NewMetricsCollector()

	// Test initial state
	metrics := mc.GetMetrics()
	if metrics == nil {
		t.Fatal("GetMetrics() returned nil")
	}

	// Test network metrics
	mc.UpdateNetworkMetrics(5, 1000, 2000)
	networkMetrics := mc.GetNetworkMetrics()
	if networkMetrics["connected_peers"] != 5 {
		t.Errorf("Expected 5 peers, got %v", networkMetrics["connected_peers"])
	}

	// Test consensus metrics
	mc.UpdateConsensusMetrics(100, 10, 0.8)
	consensusMetrics := mc.GetConsensusMetrics()
	if consensusMetrics["block_height"] != uint64(100) {
		t.Errorf("Expected block height 100, got %v", consensusMetrics["block_height"])
	}

	// Test block production recording
	mc.RecordBlockProduction()
	mc.RecordBlockProduction()

	// Test transaction recording
	mc.RecordTransaction()
	mc.RecordTransaction()
	mc.RecordTransaction()

	// Test performance metrics
	err := mc.UpdatePerformanceMetrics()
	if err != nil {
		t.Errorf("UpdatePerformanceMetrics() failed: %v", err)
	}

	// Test storage metrics
	mc.UpdateStorageMetrics(1024*1024, 1000, 500, 20)
	storageMetrics := mc.GetStorageMetrics()
	if storageMetrics["database_size_bytes"] != uint64(1024*1024) {
		t.Errorf("Expected database size 1048576, got %v", storageMetrics["database_size_bytes"])
	}

	// Test sync metrics
	mc.UpdateSyncMetrics(true, 0.5, 100.0, 3)
	syncMetrics := mc.GetSyncMetrics()
	if !syncMetrics["syncing"].(bool) {
		t.Error("Expected syncing to be true")
	}

	// Test finality time recording
	mc.RecordFinalityTime(250 * time.Millisecond)
	consensusMetrics = mc.GetConsensusMetrics()
	if consensusMetrics["finality_time"] != 0.25 {
		t.Errorf("Expected finality time 0.25, got %v", consensusMetrics["finality_time"])
	}

	// Test error counters
	mc.IncrementConnectionErrors()
	mc.IncrementSyncErrors()
	mc.IncrementForkCount()

	// Get final metrics
	finalMetrics := mc.GetMetrics()
	if finalMetrics == nil {
		t.Fatal("Final GetMetrics() returned nil")
	}

	// Verify metrics structure
	requiredSections := []string{"network", "consensus", "storage", "performance", "sync"}
	for _, section := range requiredSections {
		if _, exists := finalMetrics[section]; !exists {
			t.Errorf("Missing metrics section: %s", section)
		}
	}

	// Test reset functionality
	mc.Reset()
	resetMetrics := mc.GetMetrics()
	networkMetrics = resetMetrics["network"].(map[string]interface{})
	if networkMetrics["peer_count"] != 0 {
		t.Errorf("Expected 0 peers after reset, got %v", networkMetrics["peer_count"])
	}
}

func TestMetricsTPS(t *testing.T) {
	mc := NewMetricsCollector()

	// Record some blocks with time intervals
	mc.RecordBlockProduction()
	time.Sleep(100 * time.Millisecond)
	mc.RecordBlockProduction()
	time.Sleep(100 * time.Millisecond)
	mc.RecordBlockProduction()

	// Record some transactions
	mc.RecordTransaction()
	mc.RecordTransaction()
	mc.RecordTransaction()
	mc.RecordTransaction()
	mc.RecordTransaction()

	// Update performance metrics to calculate TPS
	mc.UpdatePerformanceMetrics()

	// Get metrics
	performanceMetrics := mc.GetPerformanceMetrics()
	blockTPS := performanceMetrics["block_tps"].(float64)
	transactionTPS := performanceMetrics["transaction_tps"].(float64)

	// Verify TPS calculations are reasonable (should be > 0)
	if blockTPS <= 0 {
		t.Errorf("Expected positive block TPS, got %f", blockTPS)
	}
	if transactionTPS <= 0 {
		t.Errorf("Expected positive transaction TPS, got %f", transactionTPS)
	}

	t.Logf("Block TPS: %f, Transaction TPS: %f", blockTPS, transactionTPS)
}
