package chaperone

func NewEnvelope[T Message](message *T, numRetries int) *Envelope[T] {
	return &Envelope[T]{
		message:    message,
		numRetries: numRetries,
	}
}
