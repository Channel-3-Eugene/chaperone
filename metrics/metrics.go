package metrics

import (
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/caio/go-tdigest/v4"
)

type Metrics struct {
	throughputCount uint64

	// For latency percentiles
	latencyDigest *tdigest.TDigest
	latencyCount  uint64

	Enabled bool
	mutex   sync.Mutex
}

func NewMetrics(enabled bool) *Metrics {
	var digest *tdigest.TDigest
	if enabled {
		digest, _ = tdigest.New()
	}
	return &Metrics{
		Enabled:       enabled,
		latencyDigest: digest,
	}
}

func (m *Metrics) IncrementThroughput() {
	if !m.Enabled {
		return
	}
	atomic.AddUint64(&m.throughputCount, 1)
}

func (m *Metrics) AddLatency(duration time.Duration) {
	if !m.Enabled {
		return
	}

	x := duration.Seconds() * 1000 // Convert to milliseconds

	m.mutex.Lock()
	defer m.mutex.Unlock()

	m.latencyCount++
	if err := m.latencyDigest.Add(x); err != nil {
		// Handle the error, e.g., log it
		log.Printf("Error adding value to tdigest: %v", err)
	}
}

func (m *Metrics) GetThroughput() uint64 {
	return atomic.LoadUint64(&m.throughputCount)
}

func (m *Metrics) GetAverageLatencyMs() float64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.latencyCount == 0 {
		return 0
	}

	var sum float64
	var totalWeight uint64

	m.latencyDigest.ForEachCentroid(func(mean float64, count uint64) bool {
		sum += mean * float64(count)
		totalWeight += count
		return true // Continue iterating
	})

	if totalWeight == 0 {
		return 0
	}

	return sum / float64(totalWeight)
}

func (m *Metrics) GetLatencyPercentile(p float64) float64 {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.latencyCount == 0 {
		return 0
	}

	return m.latencyDigest.Quantile(p / 100.0)
}
