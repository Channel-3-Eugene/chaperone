package chaperone

import (
	"context"
	"sync"

	"github.com/Channel-3-Eugene/chaperone/metrics"
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
	Stop()
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
	Close()
}

type Edge struct {
	name      string
	channel   chan Message
	closeOnce sync.Once
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
	RestartWorkers(context.Context)
	Stop(*Event)
	GetMetrics() *Metrics
}

type Node[In, Out Message] struct {
	name            string
	Handler         EnvHandler
	LoopbackHandler EnvHandler
	WorkerPool      map[string][]*Worker
	WorkerCounter   uint64
	RunningWorkers  int64

	In     map[string]MessageCarrier
	Out    OutMux
	Events MessageCarrier

	metrics *metrics.Metrics

	ctx    context.Context
	cancel context.CancelFunc

	once  sync.Once
	mutex sync.Mutex
}

type EventWorker interface { // Supervisor
	Name() string
	AddChildSupervisor(EventWorker)
	SetEvents(MessageCarrier)
	SetCancel(context.CancelFunc)
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

type Metrics struct {
	NodeName   string
	BitRate    uint64 // bits per second
	PacketRate uint64 // packets per second
	ErrorRate  uint64 // errors per second
	AvgDepth   uint64 // packets waiting in queue
}
