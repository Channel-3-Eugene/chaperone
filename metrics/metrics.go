package metrics

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	bits         uint64
	bitRate      uint64
	packetCount  uint64
	packetRate   uint64
	errorCount   uint64
	errorRate    uint64
	depthSamples []uint64
	avgDepth     uint64
	cancel       context.CancelFunc
	sampleMutex  sync.Mutex // Mutex to protect the depthSamples slice
}

// NewMetrics creates a new Metrics instance and starts a goroutine to update bitRate, packetRate, errorRate, and avgDepth every interval.
func NewMetrics(tickerInterval time.Duration) *Metrics {
	m := &Metrics{
		depthSamples: make([]uint64, 0), // Initialize the slice with some capacity
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.cancel = cancel

	// Get the ticker interval in nanoseconds
	intervalNanoseconds := tickerInterval.Nanoseconds()

	// Start the goroutine that updates the metrics every `tickerInterval`
	go func() {
		ticker := time.NewTicker(tickerInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				// Safely load the bits and calculate bitRate
				bits := atomic.LoadUint64(&m.bits)
				bitsPerSecond := (bits * 1_000_000_000) / uint64(intervalNanoseconds)
				atomic.StoreUint64(&m.bitRate, bitsPerSecond)

				// Safely load packet count and calculate packets per second
				packets := atomic.LoadUint64(&m.packetCount)
				packetsPerSecond := (packets * 1_000_000_000) / uint64(intervalNanoseconds)
				atomic.StoreUint64(&m.packetRate, packetsPerSecond)

				// Safely load error count and calculate errors per second
				errors := atomic.LoadUint64(&m.errorCount)
				errorsPerSecond := (errors * 1_000_000_000) / uint64(intervalNanoseconds)
				atomic.StoreUint64(&m.errorRate, errorsPerSecond)

				// Use a mutex to safely access the depthSamples slice and calculate avgDepth
				m.sampleMutex.Lock()
				if len(m.depthSamples) > 0 {
					var sum uint64
					for _, v := range m.depthSamples {
						sum += v
					}

					avgDepth := sum / uint64(len(m.depthSamples))
					atomic.StoreUint64(&m.avgDepth, avgDepth) // Store avgDepth atomically
					// Reset the slice length to reuse the capacity
					m.depthSamples = m.depthSamples[:0]
				} else {
					atomic.StoreUint64(&m.avgDepth, 0) // Store 0 atomically
				}
				m.sampleMutex.Unlock()

				// Reset the bits, packet, and error counters atomically
				atomic.StoreUint64(&m.bits, 0)
				atomic.StoreUint64(&m.packetCount, 0)
				atomic.StoreUint64(&m.errorCount, 0)

			case <-ctx.Done():
				// Context is canceled, exit the goroutine
				return
			}
		}
	}()

	return m
}

// Stop gracefully stops the goroutine by canceling the context
func (m *Metrics) Stop() {
	if m.cancel != nil {
		m.cancel()
	}
}

// AddBits increments the bit counter by the specified amount
func (m *Metrics) AddBits(bits uint64) {
	atomic.AddUint64(&m.bits, bits)
}

// AddPacket increments the packet counter by the specified amount
func (m *Metrics) AddPacket(count uint64) {
	atomic.AddUint64(&m.packetCount, count)
}

// AddError increments the error counter by the specified amount
func (m *Metrics) AddError(count uint64) {
	atomic.AddUint64(&m.errorCount, count)
}

// AddSample adds a depth sample to be averaged
func (m *Metrics) AddSample(sample uint64) {
	// Use a mutex to protect concurrent access to the slice
	m.sampleMutex.Lock()
	m.depthSamples = append(m.depthSamples, sample)
	m.sampleMutex.Unlock()
}

// GetBitRate returns the current bits per second rate
func (m *Metrics) GetBitRate() uint64 {
	return atomic.LoadUint64(&m.bitRate) // Use atomic load for bitRate
}

// GetPacketRate returns the current packets per second rate
func (m *Metrics) GetPacketRate() uint64 {
	return atomic.LoadUint64(&m.packetRate) // Use atomic load for packetRate
}

// GetErrorRate returns the current errors per second rate
func (m *Metrics) GetErrorRate() uint64 {
	return atomic.LoadUint64(&m.errorRate) // Use atomic load for errorRate
}

// GetAvgDepth returns the current average depth
func (m *Metrics) GetAvgDepth() uint64 {
	return atomic.LoadUint64(&m.avgDepth) // Use atomic load for avgDepth
}
