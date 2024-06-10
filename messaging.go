package chaperone

import "context"

const DEVNULL = -1

// ChannelInterface defines the interface for a channel that can send and receive envelopes.
type ChannelInterface[E Envelope[I], I any] interface {
	Send(*E)
	Receive() *E
	Destroy()
}

// EnvelopeInterface defines the interface for an envelope that wraps an item.
type EnvelopeInterface[I any] interface {
	DecrementRetry()
	Item() *I
	SetItem(*I)
	RetryCount() int
	SetRetryCount(int)
}

// Channel is a type alias for a channel of envelopes.
type Channel[E *Envelope[I], I any] chan E

// newDevNullChannel creates a new channel that discards all received messages.
func NewDevNullChannel[E *Envelope[I], I any](ctx context.Context) Channel[E, I] {
	dnchan := make(Channel[E, I])

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-dnchan:
				if !ok {
					return
				}
			}
		}
	}()

	return dnchan
}

// Envelope represents an envelope that wraps an item and tracks retry count.
type Envelope[I any] struct {
	item       I
	inputChan  int
	retryCount int
}

// NewEnvelope creates a new envelope with the given item and retry count.
func NewEnvelope[I any](item I, retryCount int) *Envelope[I] {
	return &Envelope[I]{
		item:       item,
		retryCount: retryCount,
	}
}

// DecrementRetry decrements the retry count.
func (e *Envelope[I]) DecrementRetry() {
	if e.retryCount > 0 {
		e.retryCount--
	}
}

// Item returns the item of the envelope.
func (e *Envelope[I]) Item() I {
	return e.item
}

// SetItem sets the item of the envelope.
func (e *Envelope[I]) SetItem(item I) {
	e.item = item
}

// RetryCount returns the retry count of the envelope.
func (e *Envelope[I]) RetryCount() int {
	return e.retryCount
}

// SetRetryCount sets the retry count of the envelope.
func (e *Envelope[I]) SetRetryCount(retryCount int) {
	e.retryCount = retryCount
}

// Create a new channel that can send and receive envelopes.
func NewChannel[E *Envelope[I], I any](key int) Channel[E, I] {
	return make(Channel[E, I])
}

// Send sends an envelope on the channel.
func (ch Channel[E, I]) Send(e E) {
	ch <- e
}

// Receive receives an envelope from the channel.
func (ch Channel[E, I]) Receive() E {
	e := <-ch
	return e
}

// Destroy closes the channel.
func (ch Channel[E, I]) Destroy() {
	close(ch)
}
