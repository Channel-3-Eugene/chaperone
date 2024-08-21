package chaperone

import "fmt"

func NewEdge(name string, fn EnvelopeWorker, tn EnvelopeWorker, bufferSize int, numWorkers int) *Edge {
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
	e.channel <- env
	return nil
}
