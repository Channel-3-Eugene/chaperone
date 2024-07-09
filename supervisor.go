package chaperone

import (
	"context"
	"fmt"
	"runtime"
)

func NewSupervisor[T Message](ctx context.Context, name string, handler Handler[T]) *Supervisor[T] {
	c, cancel := context.WithCancelCause(ctx)
	return &Supervisor[T]{
		ctx:         c,
		cancel:      cancel,
		Name:        name,
		Supervisors: make(map[string]*Supervisor[T]),
		Nodes:       make(map[string]*Node[T]),
		Events:      make(chan *Event[T], 1000),
		Handler:     handler,
	}
}

func (s *Supervisor[T]) AddSupervisor(supervisor *Supervisor[T]) {
	supervisor.Parent = s
	s.Supervisors[supervisor.Name] = supervisor
}

func (s *Supervisor[T]) AddNode(node *Node[T]) {
	s.Nodes[node.Name] = node
	node.EventChan = s.Events
}

func (s *Supervisor[T]) Start() {

	go func() {
		defer func() {
			if r := recover(); r != nil {
				switch x := r.(type) {
				case *Event[T]:
					ev := NewEvent(ErrorLevelCritical, fmt.Errorf("supervisor %s panicked", s.Name), x.Envelope)
					s.handleSupervisorEvent(ev)
				case runtime.Error:
					ev := NewEvent[T](ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", s.Name, x), nil)
					s.handleSupervisorEvent(ev)
				default:
					ev := NewEvent[T](ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", s.Name, x), nil)
					s.handleSupervisorEvent(ev)
				}
			}
		}()

		for {
			select {
			case event := <-s.Parent.Events:
				switch event.Level {
				case ErrorLevelCritical:
					fmt.Printf("Critical error: %#v\n", event.Envelope.Message) // TODO: replace with logging
					panic(event)
				case ErrorLevelWarning:
					fmt.Printf("Warning: %#v\n", event.Envelope.Message)
				case ErrorLevelInfo:
					fmt.Printf("Info: %#v`\n", event.Envelope.Message)
				}
			case <-s.ctx.Done():
				return
			}
		}
	}()

	// Start all the supervisors
	for _, supervisor := range s.Supervisors {
		go supervisor.Start()
	}

	// Start all the nodes
	for _, node := range s.Nodes {
		go node.Start()
	}
}

func (s *Supervisor[T]) handleSupervisorEvent(ev *Event[T]) {
	if s.Parent.Events != nil && ev.Level >= ErrorLevelError {
		s.Parent.Events <- ev
	}
}

func (s *Supervisor[T]) Stop() {
	ev := NewEvent[T](ErrorLevelInfo, fmt.Errorf("stopping supervisor %s", s.Name), nil)

	for _, supervisor := range s.Supervisors {
		supervisor.Stop()
	}

	for _, node := range s.Nodes {
		node.Stop()
	}

	if s.Parent.Events != nil {
		s.Parent.Events <- ev
		close(s.Parent.Events)
	}

	s.cancel(ev)
}
