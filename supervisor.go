package chaperone

func NewSupervisor[T Message](name string) *Supervisor[T] {
	return &Supervisor[T]{
		Name:   name,
		Nodes:  make(map[string]*Node[T]),
		events: make(chan *Event[T], 100),
	}
}

func (s *Supervisor[T]) AddNode(node *Node[T]) {
	s.Nodes[node.Name] = node
	node.eventChan = s.events
}
