package main

import (
	"runtime"
	"sync"
	"time"
)

// MetricsCollector collects and maintains system metrics
type MetricsCollector struct {
	mu sync.RWMutex

	// Network metrics
	messageLatency    map[string]time.Duration
	bandwidthIn       uint64
	bandwidthOut      uint64
	connectionErrors  uint64
	peerCount         int

	// Consensus metrics
	blockHeight       uint64
	blockTime         time.Duration
	committeeSize     int
	approvalRate      float64
	forkCount         uint64
	finalityTime      time.Duration

	// Storage metrics
	databaseSizeBytes uint64
	transactionCount  uint64
	blockCount        uint64
	validatorCount    int

	// Performance metrics
	memoryUsageBytes  uint64
	cpuUsagePercent   float64
	goroutineCount    int
	transactionTPS    float64
	blockTPS          float64

	// Sync metrics
	syncing           bool
	syncProgress      float64
	syncSpeed         float64
	peersSyncing      int
	syncErrors        uint64

	// Historical data for calculations
	blockTimes        []time.Time
	transactionTimes  []time.Time
	lastUpdate        time.Time
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		messageLatency: make(map[string]time.Duration),
		blockTimes:     make([]time.Time, 0, 100),
		transactionTimes: make([]time.Time, 0, 1000),
		lastUpdate:     time.Now(),
	}
}

// UpdateNetworkMetrics updates network-related metrics
func (mc *MetricsCollector) UpdateNetworkMetrics(peerCount int, bandwidthIn, bandwidthOut uint64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.peerCount = peerCount
	mc.bandwidthIn = bandwidthIn
	mc.bandwidthOut = bandwidthOut
}

// RecordMessageLatency records latency for a specific message type
func (mc *MetricsCollector) RecordMessageLatency(messageType string, latency time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.messageLatency[messageType] = latency
}

// IncrementConnectionErrors increments the connection error counter
func (mc *MetricsCollector) IncrementConnectionErrors() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.connectionErrors++
}

// UpdateConsensusMetrics updates consensus-related metrics
func (mc *MetricsCollector) UpdateConsensusMetrics(height uint64, committeeSize int, approvalRate float64) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.blockHeight = height
	mc.committeeSize = committeeSize
	mc.approvalRate = approvalRate
}

// RecordBlockProduction records a new block production
func (mc *MetricsCollector) RecordBlockProduction() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	now := time.Now()
	mc.blockTimes = append(mc.blockTimes, now)
	mc.blockCount++
	
	// Keep only last 100 block times for TPS calculation
	if len(mc.blockTimes) > 100 {
		mc.blockTimes = mc.blockTimes[1:]
	}
	
	// Calculate block time if we have at least 2 blocks
	if len(mc.blockTimes) >= 2 {
		mc.blockTime = mc.blockTimes[len(mc.blockTimes)-1].Sub(mc.blockTimes[len(mc.blockTimes)-2])
	}
	
	// Calculate block TPS
	if len(mc.blockTimes) >= 2 {
		timeSpan := mc.blockTimes[len(mc.blockTimes)-1].Sub(mc.blockTimes[0])
		if timeSpan > 0 {
			mc.blockTPS = float64(len(mc.blockTimes)-1) / timeSpan.Seconds()
		}
	}
}

// RecordTransaction records a new transaction
func (mc *MetricsCollector) RecordTransaction() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	now := time.Now()
	mc.transactionTimes = append(mc.transactionTimes, now)
	mc.transactionCount++
	
	// Keep only last 1000 transaction times for TPS calculation
	if len(mc.transactionTimes) > 1000 {
		mc.transactionTimes = mc.transactionTimes[1:]
	}
	
	// Calculate transaction TPS
	if len(mc.transactionTimes) >= 2 {
		timeSpan := mc.transactionTimes[len(mc.transactionTimes)-1].Sub(mc.transactionTimes[0])
		if timeSpan > 0 {
			mc.transactionTPS = float64(len(mc.transactionTimes)-1) / timeSpan.Seconds()
		}
	}
}

// UpdateStorageMetrics updates storage-related metrics
func (mc *MetricsCollector) UpdateStorageMetrics(databaseSize, transactionCount, blockCount uint64, validatorCount int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.databaseSizeBytes = databaseSize
	mc.transactionCount = transactionCount
	mc.blockCount = blockCount
	mc.validatorCount = validatorCount
}

// UpdatePerformanceMetrics updates system performance metrics
func (mc *MetricsCollector) UpdatePerformanceMetrics() error {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	// Get memory usage (simplified version without external dependencies)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	mc.memoryUsageBytes = m.Alloc
	
	// Get goroutine count
	mc.goroutineCount = runtime.NumGoroutine()
	
	// CPU usage would require external library like gopsutil
	// For now, we'll use a simplified approach
	mc.cpuUsagePercent = 0 // TODO: Implement with gopsutil if needed
	
	mc.lastUpdate = time.Now()
	return nil
}

// UpdateSyncMetrics updates sync-related metrics
func (mc *MetricsCollector) UpdateSyncMetrics(syncing bool, progress, speed float64, peersSyncing int) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.syncing = syncing
	mc.syncProgress = progress
	mc.syncSpeed = speed
	mc.peersSyncing = peersSyncing
}

// IncrementSyncErrors increments the sync error counter
func (mc *MetricsCollector) IncrementSyncErrors() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.syncErrors++
}

// IncrementForkCount increments the fork counter
func (mc *MetricsCollector) IncrementForkCount() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.forkCount++
}

// RecordFinalityTime records the time it took to finalize a block
func (mc *MetricsCollector) RecordFinalityTime(finalityTime time.Duration) {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.finalityTime = finalityTime
}

// GetMetrics returns a copy of all current metrics
func (mc *MetricsCollector) GetMetrics() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	// Calculate average message latency
	var avgLatency float64
	if len(mc.messageLatency) > 0 {
		totalLatency := time.Duration(0)
		for _, latency := range mc.messageLatency {
			totalLatency += latency
		}
		avgLatency = float64(totalLatency) / float64(len(mc.messageLatency))
	}
	
	// Calculate average block time
	var avgBlockTime float64
	if len(mc.blockTimes) >= 2 {
		timeSpan := mc.blockTimes[len(mc.blockTimes)-1].Sub(mc.blockTimes[0])
		if timeSpan > 0 {
			avgBlockTime = timeSpan.Seconds() / float64(len(mc.blockTimes)-1)
		}
	}
	
	return map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
		"network": map[string]interface{}{
			"peer_count":        mc.peerCount,
			"message_latency":   avgLatency,
			"bandwidth_in":      mc.bandwidthIn,
			"bandwidth_out":     mc.bandwidthOut,
			"connection_errors": mc.connectionErrors,
		},
		"consensus": map[string]interface{}{
			"block_height":   mc.blockHeight,
			"block_time":     avgBlockTime,
			"committee_size": mc.committeeSize,
			"approval_rate":  mc.approvalRate,
			"fork_count":     mc.forkCount,
			"finality_time":  mc.finalityTime.Seconds(),
		},
		"storage": map[string]interface{}{
			"database_size_bytes": mc.databaseSizeBytes,
			"transaction_count":   mc.transactionCount,
			"block_count":        mc.blockCount,
			"validator_count":    mc.validatorCount,
		},
		"performance": map[string]interface{}{
			"memory_usage_bytes": mc.memoryUsageBytes,
			"cpu_usage_percent":  mc.cpuUsagePercent,
			"goroutine_count":    mc.goroutineCount,
			"transaction_tps":    mc.transactionTPS,
			"block_tps":          mc.blockTPS,
		},
		"sync": map[string]interface{}{
			"syncing":       mc.syncing,
			"sync_progress": mc.syncProgress,
			"sync_speed":    mc.syncSpeed,
			"peers_syncing": mc.peersSyncing,
			"sync_errors":   mc.syncErrors,
		},
	}
}

// GetNetworkMetrics returns network-specific metrics
func (mc *MetricsCollector) GetNetworkMetrics() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	var avgLatency float64
	if len(mc.messageLatency) > 0 {
		totalLatency := time.Duration(0)
		for _, latency := range mc.messageLatency {
			totalLatency += latency
		}
		avgLatency = float64(totalLatency) / float64(len(mc.messageLatency))
	}
	
	return map[string]interface{}{
		"connected_peers":    mc.peerCount,
		"message_latency":    avgLatency,
		"bandwidth_in":       mc.bandwidthIn,
		"bandwidth_out":      mc.bandwidthOut,
		"connection_errors":  mc.connectionErrors,
	}
}

// GetConsensusMetrics returns consensus-specific metrics
func (mc *MetricsCollector) GetConsensusMetrics() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	var avgBlockTime float64
	if len(mc.blockTimes) >= 2 {
		timeSpan := mc.blockTimes[len(mc.blockTimes)-1].Sub(mc.blockTimes[0])
		if timeSpan > 0 {
			avgBlockTime = timeSpan.Seconds() / float64(len(mc.blockTimes)-1)
		}
	}
	
	return map[string]interface{}{
		"block_height":   mc.blockHeight,
		"block_time":     avgBlockTime,
		"committee_size": mc.committeeSize,
		"approval_rate":  mc.approvalRate,
		"fork_count":     mc.forkCount,
		"finality_time":  mc.finalityTime.Seconds(),
	}
}

// GetPerformanceMetrics returns performance-specific metrics
func (mc *MetricsCollector) GetPerformanceMetrics() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	return map[string]interface{}{
		"memory_usage_bytes": mc.memoryUsageBytes,
		"cpu_usage_percent":  mc.cpuUsagePercent,
		"goroutine_count":    mc.goroutineCount,
		"transaction_tps":    mc.transactionTPS,
		"block_tps":          mc.blockTPS,
	}
}

// GetStorageMetrics returns storage-specific metrics
func (mc *MetricsCollector) GetStorageMetrics() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	return map[string]interface{}{
		"database_size_bytes": mc.databaseSizeBytes,
		"transaction_count":   mc.transactionCount,
		"block_count":        mc.blockCount,
		"validator_count":    mc.validatorCount,
	}
}

// GetSyncMetrics returns sync-specific metrics
func (mc *MetricsCollector) GetSyncMetrics() map[string]interface{} {
	mc.mu.RLock()
	defer mc.mu.RUnlock()
	
	return map[string]interface{}{
		"syncing":       mc.syncing,
		"sync_progress": mc.syncProgress,
		"sync_speed":    mc.syncSpeed,
		"peers_syncing": mc.peersSyncing,
		"sync_errors":   mc.syncErrors,
	}
}

// Reset resets all metrics to zero
func (mc *MetricsCollector) Reset() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	
	mc.messageLatency = make(map[string]time.Duration)
	mc.bandwidthIn = 0
	mc.bandwidthOut = 0
	mc.connectionErrors = 0
	mc.peerCount = 0
	mc.blockHeight = 0
	mc.blockTime = 0
	mc.committeeSize = 0
	mc.approvalRate = 0
	mc.forkCount = 0
	mc.finalityTime = 0
	mc.databaseSizeBytes = 0
	mc.transactionCount = 0
	mc.blockCount = 0
	mc.validatorCount = 0
	mc.memoryUsageBytes = 0
	mc.cpuUsagePercent = 0
	mc.goroutineCount = 0
	mc.transactionTPS = 0
	mc.blockTPS = 0
	mc.syncing = false
	mc.syncProgress = 0
	mc.syncSpeed = 0
	mc.peersSyncing = 0
	mc.syncErrors = 0
	mc.blockTimes = make([]time.Time, 0, 100)
	mc.transactionTimes = make([]time.Time, 0, 1000)
	mc.lastUpdate = time.Now()
}
