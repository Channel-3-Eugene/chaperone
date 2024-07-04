package chaperone

import (
	"context"
)

type Message any

type Envelope[T Message] struct {
	message    *T
	inChan     chan *Envelope[T]
	numRetries int
}

type Handler[T Message] interface {
	Handle(msg *T) (outChName string, ev error)
}

type Worker[T Message] struct {
	ctx     context.Context
	cancel  context.CancelCauseFunc
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
	Name    string
	inChans map[string]chan *Envelope[T]
	goChans map[string]chan *Envelope[T]
}

type Node[T Message] struct {
	ctx           context.Context
	cancel        context.CancelCauseFunc
	Name          string
	handler       Handler[T]
	workerPool    map[string][]Worker[T] // Updated to map channels to workers
	workerCounter uint64

	inputChans  map[string]chan *Envelope[T]
	outputChans map[string]OutMux[T]
	devNull     chan *Envelope[T]
	eventChan   chan *Event[T]
}

type Supervisor[T Message] struct {
	Name    string
	Nodes   map[string]*Node[T]
	events  chan *Event[T]
	handler Handler[T]
}

type Event[T Message] struct {
	Level   ErrorLevel
	Event   error
	Message *T
	Worker  string
}

type Graph[T Message] struct {
	ctx         context.Context
	cancel      context.CancelFunc
	Supervisors map[string]*Supervisor[T]
	Nodes       map[string]*Node[T]
	Edges       []*Edge[T]
}
