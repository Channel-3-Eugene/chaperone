package chaperone

import (
	"context"
)

type Message interface {
	String() string
}

type Event[T Message] struct {
	Level    ErrorLevel
	Event    error
	Envelope *Envelope[T]
	Worker   string
}

type Envelope[T Message] struct {
	CurrentNode *Node[T]
	Message     T
	Event       *Event[T]
	InChan      chan *Envelope[T]
	OutChan     *OutMux[T]
	NumRetries  int
}

type Handler[T Message] interface {
	Handle(ctx context.Context, env *Envelope[T]) error
}

type Worker[T Message] struct {
	ctx     context.Context
	name    string
	handler Handler[T]
	channel chan *Envelope[T] // Each worker listens to a specific channel
}

type Edge[T Message] struct {
	Source      string
	Destination string
	Channel     chan *Envelope[T]
}

type OutMux[T Message] struct {
	Name     string
	OutChans map[string]chan *Envelope[T]
	GoChans  map[string]chan *Envelope[T]
}

type Node[T Message] struct {
	ctx           context.Context
	cancel        context.CancelCauseFunc
	Name          string
	Handler       Handler[T]
	WorkerPool    map[string][]*Worker[T] // Updated to map channels to workers
	WorkerCounter uint64

	InputChans  map[string]chan *Envelope[T]
	OutputChans map[string]*OutMux[T]
	EventChan   chan *Event[T]
}

type Supervisor[T Message] struct {
	ctx         context.Context
	cancel      context.CancelCauseFunc
	Name        string
	Nodes       map[string]*Node[T]
	Events      chan *Event[T]
	Supervisors map[string]*Supervisor[T]
	Parent      *Supervisor[T]
	Handler     Handler[T]
}

type Config struct {
	Debug bool
}

type Graph[T Message] struct {
	Config      *Config
	ctx         context.Context
	cancel      context.CancelFunc
	Name        string
	Supervisors map[string]*Supervisor[T]
	Nodes       map[string]*Node[T]
	Edges       []*Edge[T]
}
