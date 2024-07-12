package chaperone

import "fmt"

func NewEdge[FNI, FNOTNI, TNO Message](name string, fn *Node[FNI, FNOTNI], tn *Node[FNOTNI, TNO], bufferSize int, numWorkers int) *Edge[FNOTNI] {
	edge := &Edge[FNOTNI]{
		name:    name,
		Channel: make(chan *Envelope[FNOTNI], bufferSize),
	}

	// Set up the output channel on the fromNode
	fn.AddOutput(edge)

	// Set up the input channel and workers on the toNode
	tn.AddInput(name, edge)
	tn.AddWorkers(edge, numWorkers, fmt.Sprintf("%s-worker", name))

	return edge
}
