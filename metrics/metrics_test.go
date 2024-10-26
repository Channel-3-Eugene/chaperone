package metrics_test

import (
	"math"
	"sync"
	"testing"
	"time"

	"github.com/Channel-3-Eugene/chaperone/metrics"
)

func TestMetrics_NewMetrics(t *testing.T) {
	// Test when metrics are enabled
	mEnabled := metrics.NewMetrics(true)
	if !mEnabled.Enabled {
		t.Error("Expected Metrics to be enabled")
	}
	if mEnabled.GetThroughput() != 0 {
		t.Error("Expected initial throughput to be 0")
	}
	if mEnabled.GetAverageLatencyMs() != 0 {
		t.Error("Expected initial average latency to be 0")
	}

	// Test when metrics are disabled
	mDisabled := metrics.NewMetrics(false)
	if mDisabled.Enabled {
		t.Error("Expected Metrics to be disabled")
	}
}

func TestMetrics_IncrementThroughput(t *testing.T) {
	m := metrics.NewMetrics(true)

	// Test initial throughput
	if m.GetThroughput() != 0 {
		t.Errorf("Expected initial throughput to be 0, got %d", m.GetThroughput())
	}

	// Increment throughput and test
	m.IncrementThroughput()
	if m.GetThroughput() != 1 {
		t.Errorf("Expected throughput to be 1, got %d", m.GetThroughput())
	}

	m.IncrementThroughput()
	m.IncrementThroughput()
	if m.GetThroughput() != 3 {
		t.Errorf("Expected throughput to be 3, got %d", m.GetThroughput())
	}
}

func TestMetrics_AddLatencyAndAverageLatency(t *testing.T) {
	m := metrics.NewMetrics(true)

	// Add latencies
	m.AddLatency(100 * time.Millisecond)
	m.AddLatency(200 * time.Millisecond)
	m.AddLatency(300 * time.Millisecond)

	// Calculate expected average latency
	expectedAvg := (100 + 200 + 300) / 3.0

	avgLatency := m.GetAverageLatencyMs()
	if !almostEqual(avgLatency, expectedAvg) {
		t.Errorf("Expected average latency to be %.2f ms, got %.2f ms", expectedAvg, avgLatency)
	}
}

func TestMetrics_GetLatencyPercentile(t *testing.T) {
	m := metrics.NewMetrics(true)

	// Add latencies
	latencies := []time.Duration{
		100 * time.Millisecond,
		200 * time.Millisecond,
		300 * time.Millisecond,
		400 * time.Millisecond,
		500 * time.Millisecond,
	}

	for _, latency := range latencies {
		m.AddLatency(latency)
	}

	// Test 50th percentile (median)
	p50 := m.GetLatencyPercentile(50)
	expectedP50 := 300.0
	if !almostEqual(p50, expectedP50) {
		t.Errorf("Expected 50th percentile latency to be %.2f ms, got %.2f ms", expectedP50, p50)
	}

	// Test 90th percentile
	p90 := m.GetLatencyPercentile(90)
	if p90 < 450.0 || p90 > 550.0 {
		t.Errorf("Expected 90th percentile latency around 500 ms, got %.2f ms", p90)
	}

	// Test 100th percentile
	p100 := m.GetLatencyPercentile(100)
	expectedP100 := 500.0
	if !almostEqual(p100, expectedP100) {
		t.Errorf("Expected 100th percentile latency to be %.2f ms, got %.2f ms", expectedP100, p100)
	}
}

func TestMetrics_DisabledMetrics(t *testing.T) {
	m := metrics.NewMetrics(false)

	m.IncrementThroughput()
	m.AddLatency(100 * time.Millisecond)

	if m.GetThroughput() != 0 {
		t.Error("Expected throughput to be 0 when metrics are disabled")
	}

	if m.GetAverageLatencyMs() != 0 {
		t.Error("Expected average latency to be 0 when metrics are disabled")
	}

	p95 := m.GetLatencyPercentile(95)
	if p95 != 0 {
		t.Error("Expected latency percentile to be 0 when metrics are disabled")
	}
}

func TestMetrics_ZeroLatencies(t *testing.T) {
	m := metrics.NewMetrics(true)

	avgLatency := m.GetAverageLatencyMs()
	if avgLatency != 0 {
		t.Errorf("Expected average latency to be 0 when no latencies are added, got %.2f ms", avgLatency)
	}

	p95 := m.GetLatencyPercentile(95)
	if p95 != 0 {
		t.Errorf("Expected 95th percentile latency to be 0 when no latencies are added, got %.2f ms", p95)
	}
}

func TestMetrics_Concurrency(t *testing.T) {
	m := metrics.NewMetrics(true)
	var wg sync.WaitGroup

	// Simulate concurrent throughput increments and latency additions
	for i := 0; i < 1000; i++ {
		wg.Add(2)

		go func() {
			defer wg.Done()
			m.IncrementThroughput()
		}()

		go func(latency time.Duration) {
			defer wg.Done()
			m.AddLatency(latency)
		}(time.Duration(i%200) * time.Millisecond)
	}

	wg.Wait()

	if m.GetThroughput() != 1000 {
		t.Errorf("Expected throughput to be 1000, got %d", m.GetThroughput())
	}

	avgLatency := m.GetAverageLatencyMs()
	if avgLatency <= 0 {
		t.Errorf("Expected average latency to be greater than 0, got %.2f ms", avgLatency)
	}
}

// Helper function to compare floating-point numbers
func almostEqual(a, b float64) bool {
	const epsilon = 0.0001
	return math.Abs(a-b) < epsilon
}
