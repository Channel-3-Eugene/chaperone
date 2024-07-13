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

func (e ErrorLevel) String() string {
	return e.Level()
}

func NewEvent(level ErrorLevel, err error, env Message) *Event {
	if level == ErrorLevelDefault {
		level = DefaultErrorLevel
	}

	return &Event{
		level:    level,
		event:    err,
		envelope: env,
	}
}

func (e *Event) Level() ErrorLevel {
	return e.level
}

// Error implements the error interface.
func (e Event) Error() string {
	return fmt.Sprintf("[%s] %s", e.Level().String(), e.Event())
}

func (e *Event) String() string {
	return e.Error()
}

func (e *Event) Event() string {
	if e.event == nil {
		return ""
	}
	return e.event.Error()
}

func (e *Event) Message() Message {
	return e.envelope
}

func (e *Event) Wrap(err error) *Event {
	return &Event{
		level:    e.level,
		event:    err,
		envelope: e.envelope,
		Worker:   e.Worker,
	}
}

// Unwrap returns the underlying error.
func (e *Event) Unwrap() error {
	return e.event
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
