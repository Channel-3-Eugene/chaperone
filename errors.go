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

// Error wraps a standard error with additional context like error level and input item.
type Error[E *Envelope[I], I any] struct {
	Level   ErrorLevel
	Err     error
	Message *Envelope[I]
	Worker  WorkerInterface[E, I]
}

// NewError creates a new Error.
func NewError[E *Envelope[I], I any](level ErrorLevel, err error, worker WorkerInterface[E, I], message *Envelope[I]) *Error[E, I] {
	if level == ErrorLevelDefault {
		level = DefaultErrorLevel
	}

	return &Error[E, I]{
		Level:   level,
		Err:     err,
		Message: message,
		Worker:  worker,
	}
}

// Error implements the error interface.
func (e *Error[E, I]) Error() string {
	if e == nil {
		return "<nil>"
	}
	if e.Err != nil {
		return fmt.Sprintf("[%s] %#v: %v", e.Level.String(), e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %#v", e.Level.String(), e.Message)
}

// Unwrap returns the underlying error.
func (e *Error[E, I]) Unwrap() error {
	return e.Err
}

// String returns the string representation of the error level.
func (level ErrorLevel) String() string {
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
