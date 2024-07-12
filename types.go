package chaperone

import (
	"context"
)

type Message interface {
	String() string
}

type Event[In, Out Message] struct {
	Level    ErrorLevel
	Event    error
	Envelope *Envelope[In]
	Node     *Node[In, Out]
	Worker   string
}

type Envelope[T Message] struct {
	Message    T
	NumRetries int
	Metadata   map[string]interface{}
}

type EnvHandler[In, Out Message] interface {
	Start(ctx context.Context) error
	Handle(ctx context.Context, env *Envelope[In]) (*Envelope[Out], error)
}

type EvtHandler[In, Out Message] interface {
	Start(ctx context.Context) error
	Handle(ctx context.Context, evt *Event[In, Out]) error
}

type Worker[In, Out Message] struct {
	ctx     context.Context
	name    string
	handler EnvHandler[In, Out]
	channel *Edge[In] // Each worker listens to a specific channel
}

type Edge[T Message] struct {
	name    string
	Channel chan *Envelope[T]
}

type OutMux[T Message] struct {
	Name     string
	OutChans map[string]*Edge[T]
	GoChans  map[string]*Edge[T]
}

type Node[In, Out Message] struct {
	ctx           context.Context
	cancel        context.CancelFunc
	Name          string
	Handler       EnvHandler[In, Out]
	WorkerPool    map[string][]*Worker[In, Out] // Updated to map channels to workers
	WorkerCounter uint64

	In     map[string]*Edge[In]
	Out    *OutMux[Out]
	Events chan *Event[In, Out]
}

type Supervisor[In, Out Message] struct {
	ctx         context.Context
	cancel      context.CancelFunc
	Name        string
	Parent      *Supervisor[In, Out]
	Events      chan *Event[In, Out]
	Supervisors map[string]*Supervisor[In, Out]
	Nodes       map[string]*Node[In, Out]
	Handler     EvtHandler[In, Out]
}

type Config struct {
	Debug bool
}

type Graph struct {
	Config      *Config
	ctx         context.Context
	cancel      context.CancelFunc
	Name        string
	Supervisors map[string]interface{}
	Nodes       map[string]interface{}
	Edges       []interface{}
}
