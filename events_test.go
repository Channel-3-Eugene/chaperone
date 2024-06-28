package chaperone

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvents_NewEvent(t *testing.T) {
	msg := "Test message"
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
		event := NewEvent(tt.level, err, &msg)
		assert.Equal(t, tt.expected, event.Level, "Expected level to match")
		assert.Equal(t, msg, *event.Message, "Expected message to match")
		assert.Equal(t, err, event.Event, "Expected error to match")
	}
}

func TestEvents_Error(t *testing.T) {
	msg := "Test message"
	err := errors.New("Test error")
	event := NewEvent(ErrorLevelError, err, &msg)

	expected := fmt.Sprintf("[%s] %#v: %v", event.Level.Level(), event.Message, event.Event)
	assert.Equal(t, expected, event.Error(), "Expected error string to match")

	eventNoError := NewEvent(ErrorLevelInfo, nil, &msg)
	expectedNoError := fmt.Sprintf("[%s] %#v", eventNoError.Level.Level(), eventNoError.Message)
	assert.Equal(t, expectedNoError, eventNoError.Error(), "Expected error string to match when no error is present")
}

func TestEvents_Unwrap(t *testing.T) {
	msg := "Test message"
	err := errors.New("Test error")
	event := NewEvent(ErrorLevelError, err, &msg)

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
