package metrics

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func waitForCondition(t *testing.T, condition func() bool, timeout time.Duration) {
	start := time.Now()
	for {
		if condition() {
			fmt.Printf("Condition met after %v\n", time.Since(start))
			return
		}
		if time.Since(start) > timeout {
			t.Fatalf("Timeout waiting for condition")
		}
		time.Sleep(1 * time.Microsecond)
	}
}

func TestMetrics_AddBits(t *testing.T) {
	m := NewMetrics(200 * time.Microsecond) // Set faster interval for testing
	defer m.Stop()

	m.AddBits(100)
	m.AddBits(200)

	// Wait until bitRate is updated
	waitForCondition(t, func() bool {
		return m.GetBitRate() > 0
	}, 5000*time.Microsecond)

	expectedBitRate := m.GetBitRate()

	// Check that bitRate is set correctly
	assert.Equal(t, expectedBitRate, m.GetBitRate())
}

func TestMetrics_AvgDepth(t *testing.T) {
	m := NewMetrics(200 * time.Microsecond) // Set faster interval for testing
	defer m.Stop()

	m.AddSample(10)
	m.AddSample(20)
	m.AddSample(30)

	// Wait until avgDepth is updated
	waitForCondition(t, func() bool {
		return m.GetAvgDepth() == 20
	}, 5000*time.Microsecond)

	// Check that avgDepth is correctly computed
	assert.Equal(t, uint64(20), m.GetAvgDepth())
}

func TestMetrics_EmptyAvgDepth(t *testing.T) {
	m := NewMetrics(200 * time.Microsecond) // Set faster interval for testing
	defer m.Stop()

	// Wait until avgDepth is updated
	waitForCondition(t, func() bool {
		return m.GetAvgDepth() == 0
	}, 5000*time.Microsecond)

	// No samples added, avgDepth should be 0
	assert.Equal(t, uint64(0), m.GetAvgDepth())
}

func TestMetrics_Stop(t *testing.T) {
	m := NewMetrics(20 * time.Microsecond) // Set faster interval for testing
	defer m.Stop()

	m.AddBits(100)
	m.AddSample(10)

	// Wait until bitRate and avgDepth are updated
	waitForCondition(t, func() bool {
		return m.GetBitRate() > 100 && m.GetAvgDepth() == 10
	}, 500*time.Microsecond)

	expectedBitRate := m.GetBitRate()
	expectedAvgDepth := m.GetAvgDepth()

	// Stop the metrics system
	m.Stop()
	time.Sleep(20 * time.Microsecond)

	// After stopping, further operations should still be safe
	m.AddBits(50)
	m.AddSample(5)

	// Since the goroutine is stopped, values won't be updated
	// Wait and check if metrics were updated
	time.Sleep(20 * time.Microsecond)

	// Check that values before stopping are still accessible
	assert.Equal(t, expectedBitRate, m.GetBitRate())
	assert.Equal(t, expectedAvgDepth, m.GetAvgDepth())
}
