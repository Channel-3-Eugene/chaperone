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

func NewEvent[In, Out Message](level ErrorLevel, err error, env *Envelope[In]) *Event[In, Out] {
	if level == ErrorLevelDefault {
		level = DefaultErrorLevel
	}

	return &Event[In, Out]{
		Level:    level,
		Event:    err,
		Envelope: env,
	}
}

// Error implements the error interface.
func (e Event[In, Out]) Error() string {
	if e.Event != nil {
		return fmt.Sprintf("[%s] %s", e.Level.Level(), e.Event)
	}
	return fmt.Sprintf("[%s] %s", e.Level.Level(), e.Envelope.Message.String())
}

func (e *Event[In, Out]) Wrap(err error) *Event[In, Out] {
	return &Event[In, Out]{
		Level:    e.Level,
		Event:    err,
		Envelope: e.Envelope,
		Worker:   e.Worker,
	}
}

// Unwrap returns the underlying error.
func (e *Event[In, Out]) Unwrap() error {
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
