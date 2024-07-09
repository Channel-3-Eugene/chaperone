package chaperone

import (
	"context"
	"fmt"
	"sync"
)

func NewGraph[T Message](ctx context.Context, name string, config *Config) *Graph[T] {
	ctx, cancel := context.WithCancel(ctx)
	return &Graph[T]{
		ctx:         ctx,
		cancel:      cancel,
		Name:        name,
		Nodes:       make(map[string]*Node[T]),
		Supervisors: make(map[string]*Supervisor[T]),
		Edges:       make([]*Edge[T], 0),
	}
}

func (g *Graph[T]) Debug(state bool) {
	g.Config.Debug = state
}

func (g *Graph[T]) AddSupervisor(parentName *string, name string, handler Handler[T]) *Graph[T] {
	g.Supervisors[name] = NewSupervisor[T](g.ctx, name, handler)
	if parentName != nil {
		if parent, ok := g.Supervisors[*parentName]; ok {
			parent.AddSupervisor(g.Supervisors[name])
		}
	}
	return g
}

func (g *Graph[T]) AddNode(supervisorName, name string, handler Handler[T]) *Graph[T] {
	g.Nodes[name] = NewNode(g.ctx, name, handler)
	if supervisor, ok := g.Supervisors[supervisorName]; ok {
		supervisor.AddNode(g.Nodes[name])
	}
	return g
}

func (g *Graph[T]) AddEdge(fromNodeName, outchan, toNodeName, inchan string, bufferSize int, numWorkers int) *Graph[T] {
	channel := make(chan *Envelope[T], bufferSize)
	nodecount := 0
	if fromNodeName != "" && outchan != "" {
		if fromNode, ok := g.Nodes[fromNodeName]; ok {
			nodecount++
			muxName := fromNodeName + ":" + outchan
			channelName := toNodeName + ":" + inchan
			fromNode.AddOutputChannel(muxName, channelName, channel)
		}
	}
	if toNodeName != "" && inchan != "" {
		if toNode, ok := g.Nodes[toNodeName]; ok {
			nodecount++
			channelName := toNodeName + ":" + inchan
			toNode.AddInputChannel(channelName, channel)
			toNode.AddWorkers(channelName, numWorkers, fmt.Sprintf("%s-worker", channelName))
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
			for _, workers := range n.WorkerPool {
				for _, worker := range workers {
					go n.startWorker(worker)
				}
			}
		}(node)
	}
	wg.Wait() // Ensure all nodes are started
	return g
}

func (g *Graph[T]) Stop() {
	g.cancel()
}
