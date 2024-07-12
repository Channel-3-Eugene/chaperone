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
	}
}

func (g *Graph) AddSupervisor(parent EventWorker, supervisor EventWorker, nodes ...EnvelopeWorker) *Graph {
	g.Supervisors[(supervisor).Name()] = supervisor
	for _, node := range nodes {
		supervisor.AddNode(node)
	}
	return g
}

// func (g *Graph) AddSupervisor(supervisor any, nodes ...any) *Graph {
// 	s, ok := supervisor.(*Supervisor[Message, Message])
// 	if !ok {
// 		panic("supervisor is incorrect type")
// 	}
// 	g.Supervisors[s.Name] = supervisor
// 	for _, node := range nodes {
// 		n, ok := node.(*Node[Message, Message])
// 		if !ok {
// 			panic("node is incorrect type")
// 		}
// 		s.AddNode(n)
// 	}
// 	return g
// }

func (g *Graph) AddNode(node EnvelopeWorker) *Graph {
	n, ok := node.(*Node[Message, Message])
	if !ok {
		panic("node is incorrect type")
	}
	g.Nodes[n.Name()] = node
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
