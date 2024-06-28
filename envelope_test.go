package chaperone

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEnvelope_NewEnvelope(t *testing.T) {
	msg := "Test message"
	numRetries := 3

	envelope := NewEnvelope(&msg, numRetries)

	assert.NotNil(t, envelope, "Envelope should not be nil")
	assert.Equal(t, &msg, envelope.message, "Expected message to match")
	assert.Equal(t, numRetries, envelope.numRetries, "Expected numRetries to match")
	assert.Nil(t, envelope.inChan, "Expected inChan to be nil")
}
