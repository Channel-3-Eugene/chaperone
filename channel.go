package chaperone

import "fmt"

func NewChannel[T Message](bufferSize int, isDevNull bool) chan *Envelope[T] {
	ch := make(chan *Envelope[T], bufferSize)
	fmt.Printf("Created channel %#v with buffer size %d\n", ch, bufferSize)

	if isDevNull {
		go func() {
			for range ch {
				// Just consume the messages without doing anything
			}
		}()
	}

	return ch
}
