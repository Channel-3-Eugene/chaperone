package chaperone

import (
	"context"
	"slices"
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
		supervisor.SetCancel(g.cancel)
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

// Metrics returns the metrics for the given nodes
// If nodes is nil, return metrics for all nodes
// Case sensitive naming
func (g *Graph) Metrics(nodes []string) []*Metrics {
	metrics := make([]*Metrics, 0)

	for _, node := range g.Nodes {
		if nodes == nil || slices.Contains(nodes, node.Name()) {
			metrics = append(metrics, node.GetMetrics())
		}
	}

	return metrics
}
