package chaperone

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type eventTestMessage struct {
	Content string
}

func (m eventTestMessage) String() string {
	return m.Content
}

func TestEvents_NewEvent(t *testing.T) {
	env := &Envelope[eventTestMessage]{
		Message: eventTestMessage{
			Content: "Test message",
		},
	}
	err := errors.New("Test error")

	tests := []struct {
		level    ErrorLevel
		expected ErrorLevel
	}{
		{ErrorLevelDefault, DefaultErrorLevel},
		{ErrorLevelDebug, ErrorLevelDebug},
		{ErrorLevelInfo, ErrorLevelInfo},
		{ErrorLevelWarning, ErrorLevelWarning},
		{ErrorLevelError, ErrorLevelError},
		{ErrorLevelCritical, ErrorLevelCritical},
	}

	for _, tt := range tests {
		event := NewEvent(tt.level, err, env)
		assert.Equal(t, tt.expected, event.Level(), "Expected level to match")
		assert.Equal(t, env.String(), event.envelope.String(), "Expected message to match")
		assert.Equal(t, err.Error(), event.Event(), "Expected error to match")
	}
}

func TestEvents_Error(t *testing.T) {
	env := &Envelope[eventTestMessage]{
		Message: eventTestMessage{
			Content: "Test message",
		},
	}
	err := errors.New("Test error")
	event := NewEvent(ErrorLevelError, err, env)

	expected := fmt.Sprintf("[%s] %s", event.Level().String(), event.Event())
	assert.Equal(t, expected, event.Error(), "Expected error string to match")

	eventNoError := NewEvent(ErrorLevelInfo, nil, env)
	expectedNoError := fmt.Sprintf("[%s] %s", eventNoError.Level().String(), eventNoError.Event())
	assert.Equal(t, expectedNoError, eventNoError.Error(), "Expected error string to match when no error is present")
	assert.Empty(t, eventNoError.Event(), "Expected error to be nil")
}

func TestEvents_Unwrap(t *testing.T) {
	env := &Envelope[eventTestMessage]{
		Message: eventTestMessage{
			Content: "Test message",
		},
	}
	err := errors.New("Test error")
	event := NewEvent(ErrorLevelError, err, env)

	unwrapped := event.Unwrap()
	assert.Equal(t, err, unwrapped, "Expected unwrapped error to match")
}

func TestEvents_ErrorLevels(t *testing.T) {
	tests := []struct {
		level    ErrorLevel
		expected string
	}{
		{ErrorLevelDebug, "DEBUG"},
		{ErrorLevelInfo, "INFO"},
		{ErrorLevelWarning, "WARNING"},
		{ErrorLevelError, "ERROR"},
		{ErrorLevelCritical, "CRITICAL"},
		{ErrorLevel(999), "UNKNOWN"}, // Test an unknown error level
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.level.Level(), "Expected level string to match")
	}
}
