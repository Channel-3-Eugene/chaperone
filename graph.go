package chaperone

import (
	"context"
)

func NewGraph(ctx context.Context, name string, config *Config) *Graph {
	ctx, cancel := context.WithCancel(ctx)
	return &Graph{
		ctx:         ctx,
		cancel:      cancel,
		Name:        name,
		Nodes:       make(map[string]EnvelopeWorker),
		Supervisors: make(map[string]EventWorker),
		Edges:       make([]MessageCarrier, 0),
		done:        make(chan struct{}),
	}
}

func (g *Graph) AddSupervisor(parent EventWorker, supervisor EventWorker) *Graph {
	if parent == nil {
		g.Supervisors[(supervisor).Name()] = supervisor
	} else {
		parent.AddChildSupervisor(supervisor)
	}
	return g
}

func (g *Graph) AddNode(supervisor EventWorker, node EnvelopeWorker) *Graph {
	g.Nodes[node.Name()] = node
	supervisor.AddNode(node)
	return g
}

func (g *Graph) AddEdge(edge MessageCarrier) *Graph {
	g.Edges = append(g.Edges, edge)
	return g
}

func (g *Graph) Start() *Graph {
	for _, supervisor := range g.Supervisors {
		supervisor.Start()
	}
	return g
}

func (g *Graph) Stop() {
	g.cancel()
}

func (g *Graph) Done() chan struct{} {
	return g.done
}
