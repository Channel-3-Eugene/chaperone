package chaperone

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type envelopeTestMessage struct {
	Content string
}

func (m envelopeTestMessage) String() string {
	return m.Content
}

func TestEnvelope_NewEnvelope(t *testing.T) {
	msg := envelopeTestMessage{Content: "Test message"}
	numRetries := 3

	envelope := NewEnvelope(&msg, numRetries)

	assert.NotNil(t, envelope, "Envelope should not be nil")
	assert.Equal(t, &msg, envelope.Message, "Expected message to match")
	assert.Equal(t, numRetries, envelope.NumRetries, "Expected numRetries to match")
	assert.Nil(t, envelope.InChan, "Expected inChan to be nil")
}
