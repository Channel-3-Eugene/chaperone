package chaperone

import (
	"context"
	"fmt"
	"runtime"
)

func NewSupervisor[In, Out Message](ctx context.Context, name string, handler EvtHandler[In, Out]) *Supervisor[In, Out] {
	c, cancel := context.WithCancel(ctx)
	return &Supervisor[In, Out]{
		ctx:         c,
		cancel:      cancel,
		Name:        name,
		Supervisors: make(map[string]*Supervisor[In, Out]),
		Nodes:       make(map[string]*Node[In, Out]),
		Events:      make(chan *Event[In, Out], 1000),
		Handler:     handler,
	}
}

func (s *Supervisor[In, Out]) AddSupervisor(supervisor *Supervisor[In, Out]) {
	supervisor.Parent = s
	s.Supervisors[supervisor.Name] = supervisor
}

func (s *Supervisor[In, Out]) AddNode(node *Node[In, Out]) {
	s.Nodes[node.Name] = node
	node.Events = s.Events
}

func (s *Supervisor[In, Out]) Start() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				switch x := r.(type) {
				case *Event[In, Out]:
					ev := NewEvent[In, Out](ErrorLevelCritical, fmt.Errorf("supervisor %s panicked", s.Name), x.Envelope)
					s.handleSupervisorEvent(ev)
				case runtime.Error:
					ev := NewEvent[In, Out](ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", s.Name, x), nil)
					s.handleSupervisorEvent(ev)
				default:
					ev := NewEvent[In, Out](ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", s.Name, x), nil)
					s.handleSupervisorEvent(ev)
				}
			}
		}()

		for {
			select {
			case event := <-s.Events:
				switch event.Level {
				case ErrorLevelCritical:
					fmt.Printf("Critical error: %#v\n", event.Envelope.Message) // TODO: replace with logging
					panic(event)
				case ErrorLevelWarning:
					fmt.Printf("Warning: %#v\n", event.Envelope.Message)
					s.handleSupervisorEvent(event)
				case ErrorLevelInfo:
					fmt.Printf("Info: %#v\n", event.Envelope.Message)
					s.handleSupervisorEvent(event)
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

func (s *Supervisor[In, Out]) handleSupervisorEvent(ev *Event[In, Out]) {
	if ev.Level >= ErrorLevelError && s.Parent != nil && s.Parent.Events != nil {
		s.Parent.Events <- ev
	}
	if ev.Level == ErrorLevelCritical {
		fmt.Println("Critical error encountered, restarting node workers.")
		ev.Node.RestartWorkers()
	}
}

func (s *Supervisor[In, Out]) Stop() {
	ev := NewEvent[In, Out](ErrorLevelInfo, fmt.Errorf("stopping supervisor %s", s.Name), nil)

	for _, supervisor := range s.Supervisors {
		supervisor.Stop()
	}

	for _, node := range s.Nodes {
		node.Stop(ev)
	}

	if s.Parent.Events != nil {
		s.Parent.Events <- ev
		close(s.Parent.Events)
	}

	s.cancel()
}
