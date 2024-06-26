package chaperone

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestChannel_NewChannel(t *testing.T) {
	ch := NewChannel[string](10, false)
	assert.NotNil(t, ch, "Channel should not be nil")

	msg := "Test message"
	envelope := &Envelope[string]{message: &msg}
	ch <- envelope

	received := <-ch
	assert.Equal(t, envelope, received, "Expected to receive the same envelope")

	// Clean up by closing the channel
	close(ch)
}

func TestChannel_NewDevNull(t *testing.T) {
	ch := NewChannel[string](10, true)
	assert.NotNil(t, ch, "Channel should not be nil")

	msg := "Test message"
	envelope := &Envelope[string]{message: &msg}
	ch <- envelope

	// Ensure that the message is consumed and the channel does not block
	select {
	case ch <- envelope:
		// This is the expected behavior
	case <-time.After(1 * time.Millisecond):
		t.Error("Expected DevNull channel to consume the message without blocking")
	}

	// Clean up by closing the channel
	close(ch)
}
