package chaperone

import (
	"context"
)

type Message interface { // Envelope or Payload
	String() string
}

type Event struct {
	level    ErrorLevel
	event    error
	envelope Message
	node     EnvelopeWorker
	Worker   string
}

type Envelope[T Message] struct {
	Message    T
	NumRetries int
	Metadata   map[string]interface{}
}

type EnvHandler interface {
	Start(ctx context.Context) error
	Handle(ctx context.Context, env Message) (Message, error)
	Stop()
}

type EvtHandler interface {
	Start(ctx context.Context) error
	Handle(ctx context.Context, evt Message) error
}

type Worker struct {
	name      string
	listening MessageCarrier // Each worker listens to a specific channel
	handler   EnvHandler
}

type MessageCarrier interface { // Edge
	Name() string
	Send(env Message) error
	GetChannel() chan Message
	SetChannel(chan Message)
}

type Edge struct {
	name    string
	channel chan Message
}

type OutMux struct {
	Name     string
	OutChans map[string]MessageCarrier
	GoChans  map[string]MessageCarrier
}

type EnvelopeWorker interface { // Node
	Name() string
	AddOutput(MessageCarrier)
	AddInput(MessageCarrier)
	AddWorkers(MessageCarrier, int, string, EnvHandler)
	GetHandler() EnvHandler
	SetEvents(MessageCarrier)
	Start(context.Context)
	RestartWorkers()
	Stop(*Event)
}

type Node[In, Out Message] struct {
	name            string
	Handler         EnvHandler
	LoopbackHandler EnvHandler
	WorkerPool      map[string][]*Worker // Updated to map channels to workers
	WorkerCounter   uint64
	RunningWorkers  int64

	In     map[string]MessageCarrier
	Out    OutMux
	Events MessageCarrier

	ctx    context.Context
	cancel context.CancelFunc
}

type EventWorker interface { // Supervisor
	Name() string
	AddChildSupervisor(EventWorker)
	SetEvents(MessageCarrier)
	Start(ctx context.Context)
	Stop()
}

type Supervisor struct {
	name         string
	Events       MessageCarrier
	ParentEvents MessageCarrier
	Supervisors  map[string]EventWorker
	Nodes        map[string]EnvelopeWorker
	Handler      EvtHandler

	ctx    context.Context
	cancel context.CancelFunc
}

type Config struct {
	Debug bool
}

type Graph struct {
	Name        string
	Supervisors map[string]EventWorker
	Nodes       map[string]EnvelopeWorker
	Edges       []MessageCarrier

	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
}
