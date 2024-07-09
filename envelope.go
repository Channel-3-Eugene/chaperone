package chaperone

func NewEnvelope[T Message](message T, numRetries int) *Envelope[T] {
	return &Envelope[T]{
		Message:    message,
		NumRetries: numRetries,
	}
}

func (e *Envelope[T]) Receive(node *Node[T], inChan chan *Envelope[T]) {
	e.CurrentNode = node
	e.InChan = inChan
}
