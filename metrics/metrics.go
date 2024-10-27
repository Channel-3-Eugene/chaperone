package metrics

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

type Metrics struct {
	bits     uint64
	bitRate  uint64
	avgDepth uint64 // Now accessed atomically

	samples     []uint64
	cancel      context.CancelFunc
	sampleMutex sync.Mutex // Mutex to protect the samples slice
}

// NewMetrics creates a new Metrics instance and starts a goroutine to update bitRate and avgDepth every interval.
func NewMetrics(tickerInterval time.Duration) *Metrics {
	m := &Metrics{
		samples: make([]uint64, 0), // Initialize the slice with some capacity
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
				// Safely load the bits
				bits := atomic.LoadUint64(&m.bits)

				// Scale bitRate to bits per second using integer math
				// bitsPerSecond = (bits * 1e9) / intervalNanoseconds
				bitsPerSecond := (bits * 1_000_000_000) / uint64(intervalNanoseconds)
				atomic.StoreUint64(&m.bitRate, bitsPerSecond)

				// Use a mutex to safely access the samples slice
				m.sampleMutex.Lock()
				if len(m.samples) > 0 {
					var sum uint64
					for _, v := range m.samples {
						sum += v
					}

					avgDepth := sum / uint64(len(m.samples))
					atomic.StoreUint64(&m.avgDepth, avgDepth) // Store avgDepth atomically
					// Reset the slice length to reuse the capacity
					m.samples = m.samples[:0]
				} else {
					atomic.StoreUint64(&m.avgDepth, 0) // Store 0 atomically
				}
				m.sampleMutex.Unlock()

				// Reset the bits counter atomically
				atomic.StoreUint64(&m.bits, 0)

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

func (m *Metrics) AddBits(bits uint64) {
	atomic.AddUint64(&m.bits, bits)
}

func (m *Metrics) AddSample(sample uint64) {
	// Use a mutex to protect concurrent access to the slice
	m.sampleMutex.Lock()
	m.samples = append(m.samples, sample)
	m.sampleMutex.Unlock()
}

func (m *Metrics) GetBitRate() uint64 {
	return atomic.LoadUint64(&m.bitRate) // Use atomic load for bitRate
}

func (m *Metrics) GetAvgDepth() uint64 {
	return atomic.LoadUint64(&m.avgDepth) // Use atomic load for avgDepth
}
