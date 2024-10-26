package chaperone

import (
	"context"
	"fmt"
	"runtime"
)

func NewSupervisor(name string, handler EvtHandler) *Supervisor {
	return &Supervisor{
		name:        name,
		Supervisors: make(map[string]EventWorker),
		Nodes:       make(map[string]EnvelopeWorker),
		Events:      NewEdge("events", nil, nil, 1000, 1),
		Handler:     handler,
	}
}

func (s *Supervisor) Name() string {
	return s.name
}

func (s *Supervisor) AddChildSupervisor(supervisor EventWorker) {
	if supervisor != nil {
		s.Supervisors[supervisor.Name()] = supervisor
		supervisor.SetEvents(s.Events)
	}
}

func (s *Supervisor) SetEvents(edge MessageCarrier) {
	e := edge.(*Edge)
	s.ParentEvents = e
}

func (s *Supervisor) addNode(node EnvelopeWorker) {
	s.Nodes[node.Name()] = node
	node.SetEvents(s.Events)
}

func (s *Supervisor) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	s.ctx = ctx
	s.cancel = cancel

	go func() {
		defer func() {
			if r := recover(); r != nil {
				switch x := r.(type) {
				case *Event:
					ev := NewEvent(ErrorLevelCritical, fmt.Errorf("supervisor %s panicked", s.Name()), x.Message())
					s.handleSupervisorEvent(ev)
				case runtime.Error:
					ev := NewEvent(ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", s.Name(), x), nil)
					s.handleSupervisorEvent(ev)
				default:
					ev := NewEvent(ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", s.Name(), x), nil)
					s.handleSupervisorEvent(ev)
				}
			}
		}()

		evt := s.Handler.Start(ctx)
		if evt != nil {
			if e, ok := evt.(*Event); ok {
				s.handleSupervisorEvent(e)
			} else {
				newErr := NewEvent(ErrorLevelError, fmt.Errorf("supervisor %s returned invalid event", s.name), nil)
				s.handleSupervisorEvent(newErr)
			}
		}

		for {
			select {
			case msg := <-s.Events.GetChannel():
				event, ok := msg.(*Event)
				if !ok {
					fmt.Printf("Received unexpected message type: %#v\n", msg)
					continue
				}

				switch event.Level() {
				case ErrorLevelCritical:
					fmt.Printf("Critical error: %#v\n", event.Message()) // TODO: replace with logging
					panic(event)
				default:
					s.handleSupervisorEvent(event)
				}
			case <-ctx.Done():
				fmt.Printf("Supervisor %s context canceled\n", s.Name())

				return
			}
		}
	}()

	// Start all the supervisors
	for _, supervisor := range s.Supervisors {
		go supervisor.Start(ctx)
	}

	// Start all the nodes
	for _, node := range s.Nodes {
		go node.Start(ctx)
	}
}

func (s *Supervisor) handleSupervisorEvent(evt *Event) {
	if evt.Level() >= ErrorLevelError && s.ParentEvents != nil {
		s.ParentEvents.Send(evt)
	}
	if evt.Level() == ErrorLevelCritical {
		fmt.Println("Critical error encountered, restarting node workers.")
		evt.node.RestartWorkers(context.Background())
	}
}

func (s *Supervisor) Stop() {
	evt := NewEvent(ErrorLevelInfo, fmt.Errorf("stopping supervisor %s", s.Name()), nil)

	for _, supervisor := range s.Supervisors {
		supervisor.Stop()
	}

	for _, node := range s.Nodes {
		node.Stop(evt)
	}

	if s.ParentEvents != nil {
		s.ParentEvents.Send(evt)
	}

	s.cancel()

	s.Handler.Stop()
}
