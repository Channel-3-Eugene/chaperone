package chaperone

import (
	"context"
	"sync"
)

func NewGraph[T Message](ctx context.Context) *Graph[T] {
	ctx, cancel := context.WithCancel(ctx)
	return &Graph[T]{
		ctx:         ctx,
		cancel:      cancel,
		Nodes:       make(map[string]*Node[T]),
		Supervisors: make(map[string]*Supervisor[T]),
		Edges:       make([]*Edge[T], 0),
	}
}

func (g *Graph[T]) AddSupervisor(name string, handler Handler[T]) *Graph[T] {
	g.Supervisors[name] = NewSupervisor[T](name, handler)
	return g
}

func (g *Graph[T]) AddNode(supervisorName, name string, handler Handler[T]) *Graph[T] {
	c, cancel := context.WithCancelCause(g.ctx)
	g.Nodes[name] = newNode(c, cancel, name, handler)
	if supervisor, ok := g.Supervisors[supervisorName]; ok {
		supervisor.AddNode(g.Nodes[name])
	}
	return g
}

func (g *Graph[T]) AddWorkers(nodeName string, num int, name string) *Graph[T] {
	g.Nodes[nodeName].AddWorkers(num, name)
	return g
}

func (g *Graph[T]) AddEdge(fromNodeName, outchan, toNodeName, inchan string, bufferSize int) *Graph[T] {
	channel := NewChannel[T](bufferSize, false)
	nodecount := 0
	if fromNodeName != "" && outchan != "" {
		if _, ok := g.Nodes[fromNodeName]; ok {
			nodecount++
			muxName := fromNodeName + ":" + outchan
			channelName := toNodeName + ":" + inchan
			g.Nodes[fromNodeName].AddOutputChannel(muxName, channelName, channel)
		}
	}
	if toNodeName != "" && inchan != "" {
		nodecount++
		if _, ok := g.Nodes[toNodeName]; ok {
			g.Nodes[toNodeName].AddInputChannel(inchan, channel)
		}
	}
	if nodecount == 0 {
		panic("No nodes provided for edge")
	}
	g.Edges = append(g.Edges, &Edge[T]{Source: fromNodeName + ":" + outchan, Destination: toNodeName + ":" + inchan, Channel: channel})
	return g
}

func (g *Graph[T]) Start() *Graph[T] {
	var wg sync.WaitGroup
	for _, node := range g.Nodes {
		wg.Add(1)
		go func(n *Node[T]) {
			defer wg.Done()
			n.Start()
		}(node)
	}
	wg.Wait() // Ensure all nodes are started
	return g
}

func (g *Graph[T]) Stop() {
	g.cancel()
}
