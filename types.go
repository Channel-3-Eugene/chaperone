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
}

type OutMux[T Message] struct {
	outChans []chan *Envelope[T]
}

type Edge[T Message] struct {
	Source      string
	Destination string
	Channel     chan *Envelope[T]
}

type Node[T Message] struct {
	ctx           context.Context
	cancel        context.CancelCauseFunc
	Name          string
	handler       Handler[T]
	workerPool    []Worker[T]
	workerCounter uint64

	inputChans  map[string]chan *Envelope[T]
	outputChans map[string]OutMux[T]
	devNull     chan *Envelope[T]
	eventChan   chan *Event[T]
}

type Supervisor[T Message] struct {
	Name   string
	Nodes  map[string]*Node[T]
	events chan *Event[T]
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
