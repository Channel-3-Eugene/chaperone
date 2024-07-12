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
		Nodes:       make(map[string]any),
		Supervisors: make(map[string]any),
		Edges:       make([]any, 0),
	}
}

func (g *Graph) AddSupervisor(supervisor any, nodes ...any) *Graph {
	s, ok := supervisor.(*Supervisor[Message, Message])
	if !ok {
		panic("supervisor is incorrect type")
	}
	g.Supervisors[s.Name] = supervisor
	for _, node := range nodes {
		n, ok := node.(*Node[Message, Message])
		if !ok {
			panic("node is incorrect type")
		}
		s.AddNode(n)
	}
	return g
}

func (g *Graph) AddNode(node any) *Graph {
	n, ok := node.(*Node[Message, Message])
	if !ok {
		panic("node is incorrect type")
	}
	g.Nodes[n.Name] = node
	return g
}

func (g *Graph) AddEdge(edge any) *Graph {
	e, ok := edge.(*Edge[Message])
	if !ok {
		panic("edge is incorrect type")
	}
	g.Edges = append(g.Edges, e)
	return g
}

func (g *Graph) Start() *Graph {
	for _, supervisor := range g.Supervisors {
		s, ok := supervisor.(*Supervisor[Message, Message])
		if !ok {
			panic("supervisor is incorrect type")
		}
		s.Start()
	}
	return g
}

func (g *Graph) Stop() {
	g.cancel()
}
