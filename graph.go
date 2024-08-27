package chaperone

import (
	"context"
)

func NewGraph(name string, config *Config) *Graph {
	return &Graph{
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
	supe := supervisor.(*Supervisor)
	supe.addNode(node)
	return g
}

func (g *Graph) AddEdge(edge MessageCarrier) *Graph {
	g.Edges = append(g.Edges, edge)
	return g
}

func (g *Graph) Start(ctx context.Context) *Graph {
	ctx, cancel := context.WithCancel(ctx)
	g.ctx = ctx
	g.cancel = cancel

	for _, supervisor := range g.Supervisors {
		supervisor.Start(ctx)
	}
	return g
}

func (g *Graph) Stop() *Graph {
	for _, supervisor := range g.Supervisors {
		supervisor.Stop()
	}

	return g
}
