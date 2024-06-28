package chaperone

import (
	"fmt"
)

// ErrorLevel represents the severity of an error.
type ErrorLevel int

const (
	// ErrorLevelDefault selects the default error level.
	ErrorLevelDefault ErrorLevel = iota
	// ErrorLevelDebug represents a very granular message.
	ErrorLevelDebug
	// ErrorLevelInfo represents an informational message.
	ErrorLevelInfo
	// ErrorLevelWarning represents a warning message.
	ErrorLevelWarning
	// ErrorLevelError represents an error message.
	ErrorLevelError
	// ErrorLevelCritical represents a critical error message.
	ErrorLevelCritical
)

const DefaultErrorLevel = ErrorLevelInfo

func NewEvent[T Message](level ErrorLevel, err error, msg *T) Event[T] {
	if level == ErrorLevelDefault {
		level = DefaultErrorLevel
	}

	return Event[T]{
		Level:   level,
		Event:   err,
		Message: msg,
	}
}

// Error implements the error interface.
func (e Event[T]) Error() string {
	if e.Event != nil {
		return fmt.Sprintf("[%s] %#v: %v", e.Level.Level(), e.Message, e.Event)
	}
	return fmt.Sprintf("[%s] %#v", e.Level.Level(), e.Message)
}

// Unwrap returns the underlying error.
func (e *Event[T]) Unwrap() error {
	return e.Event
}

// String returns the string representation of the error level.
func (level ErrorLevel) Level() string {
	switch level {
	case ErrorLevelDebug:
		return "DEBUG"
	case ErrorLevelInfo:
		return "INFO"
	case ErrorLevelWarning:
		return "WARNING"
	case ErrorLevelError:
		return "ERROR"
	case ErrorLevelCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}
