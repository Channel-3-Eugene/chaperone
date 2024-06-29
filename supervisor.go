package chaperone

func NewSupervisor[T Message](name string, handler Handler[T]) *Supervisor[T] {
	return &Supervisor[T]{
		Name:    name,
		Nodes:   make(map[string]*Node[T]),
		events:  make(chan *Event[T], 1000),
		handler: handler,
	}
}

func (s *Supervisor[T]) AddNode(node *Node[T]) {
	s.Nodes[node.Name] = node
	node.eventChan = s.events
}
