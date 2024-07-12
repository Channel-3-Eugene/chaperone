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
}

type EvtHandler interface {
	Start(ctx context.Context) error
	Handle(ctx context.Context, evt Message) error
}

type Worker struct {
	ctx       context.Context
	name      string
	handler   EnvHandler
	listening MessageCarrier // Each worker listens to a specific channel
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
	AddInput(string, MessageCarrier)
	AddWorkers(MessageCarrier, int, string)
	SetEvents(MessageCarrier)
	Start()
	RestartWorkers()
	Stop(*Event)
}

type Node[In, Out Message] struct {
	ctx           context.Context
	cancel        context.CancelFunc
	name          string
	Handler       EnvHandler
	WorkerPool    map[string][]*Worker // Updated to map channels to workers
	WorkerCounter uint64

	In     map[string]MessageCarrier
	Out    OutMux
	Events MessageCarrier
}

type EventWorker interface { // Supervisor
	Name() string
	AddChildSupervisor(EventWorker)
	AddNode(EnvelopeWorker)
	SetEvents(MessageCarrier)
	Start()
	Stop()
}

type Supervisor struct {
	ctx          context.Context
	cancel       context.CancelFunc
	name         string
	Events       MessageCarrier
	ParentEvents MessageCarrier
	Supervisors  map[string]EventWorker
	Nodes        map[string]EnvelopeWorker
	Handler      EvtHandler
}

type Config struct {
	Debug bool
}

type Graph struct {
	ctx         context.Context
	cancel      context.CancelFunc
	Name        string
	Supervisors map[string]EventWorker
	Nodes       map[string]EnvelopeWorker
	Edges       []MessageCarrier
}
