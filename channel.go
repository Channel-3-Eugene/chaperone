package chaperone

func NewChannel[T Message](bufferSize int, isDevNull bool) chan *Envelope[T] {
	ch := make(chan *Envelope[T], bufferSize)

	if isDevNull {
		go func() {
			for range ch {
				// Just consume the messages without doing anything
			}
		}()
	}

	return ch
}
