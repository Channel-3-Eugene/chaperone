package chaperone

func NewEnvelope[T Message](message T, numRetries int) *Envelope[T] {
	return &Envelope[T]{
		Message:    message,
		NumRetries: numRetries,
		Metadata:   make(map[string]interface{}),
	}
}
