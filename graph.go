package chaperone

import (
	"context"
)

func NewGraph[T Message](ctx context.Context) *Graph[T] {
	ctx, cancel := context.WithCancel(ctx)
	return &Graph[T]{
		ctx:         ctx,
		cancel:      cancel,
		Nodes:       make(map[string]*Node[T]),
		Supervisors: make(map[string]*Supervisor[T]),
	}
}

func (g *Graph[T]) AddSupervisor(name string) {
	g.Supervisors[name] = NewSupervisor[T](name)
}

func (g *Graph[T]) AddNode(supervisor *Supervisor[T], name string, handler Handler[T], retryLimit int) {
	c, cancel := context.WithCancelCause(g.ctx)
	g.Nodes[name] = newNode(c, cancel, name, handler, retryLimit)
	if supervisor != nil {
		supervisor.AddNode(g.Nodes[name])
	}
}

func (g *Graph[T]) AddEdge(fromNodeName, outchan, toNodeName, inchan string, bufferSize int) {
	channel := NewChannel[T](bufferSize, false)
	g.Nodes[fromNodeName].AddOutputChannel(outchan, channel)
	g.Nodes[toNodeName].AddInputChannel(inchan, channel)
	g.Edges = append(g.Edges, &Edge[T]{Source: fromNodeName + ":" + outchan, Destination: toNodeName + ":" + inchan, Channel: channel})
}

func (g *Graph[T]) Start() {
	for _, node := range g.Nodes {
		node.Start()
	}
}
