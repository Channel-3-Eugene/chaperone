package chaperone

import (
	"fmt"
	"time"
)

func NewEdge(name string, fn EnvelopeWorker, tn EnvelopeWorker, bufferSize int, numWorkers int) MessageCarrier {
	edge := &Edge{
		name:    name,
		channel: make(chan Message, bufferSize),
	}

	// Set up the output channel on the fromNode
	if fn != nil {
		fn.AddOutput(edge)
	}

	// Set up the input channel and workers on the toNode
	if tn != nil {
		tn.AddInput(edge)
		tn.AddWorkers(edge, numWorkers, fmt.Sprintf("%s-worker", name), tn.GetHandler())
	}

	return edge
}

func (e *Edge) Name() string {
	return e.name
}

func (e *Edge) GetChannel() chan Message {
	return e.channel
}

func (e *Edge) SetChannel(c chan Message) {
	e.channel = c
}

func (e *Edge) Send(env Message) error {
	if e.channel == nil {
		return fmt.Errorf("channel is nil")
	}

	e.channel <- env

	return nil
}

func (e *Edge) Close() {
	// Close gracefully, ensuring it's done only once
	e.closeOnce.Do(func() {
		for len(e.channel) > 0 {
			time.Sleep(100 * time.Microsecond)
		}
		close(e.channel)
	})
}
