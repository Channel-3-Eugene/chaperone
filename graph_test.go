package chaperone

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type graphTestMessage struct {
	Content string
}

func (m graphTestMessage) String() string {
	return m.Content
}

type graphTestHandler struct{}

func (h *graphTestHandler) Start(context.Context) error {
	return nil
}

func (h *graphTestHandler) Handle(_ context.Context, env Message) (Message, error) {
	if env.String() == "error" {
		return nil, errors.New("test error")
	}
	return env, nil
}

type graphSupervisorTestHandler struct{}

func (h *graphSupervisorTestHandler) Start(context.Context) error {
	return nil
}

func (h *graphSupervisorTestHandler) Handle(_ context.Context, evt Message) error {
	return nil
}

func TestGraph_NewGraph(t *testing.T) {
	t.Run("creates a new graph with the given context", func(t *testing.T) {
		ctx := context.Background()
		graph := NewGraph(ctx, "graph", &Config{})

		assert.NotNil(t, graph)
		assert.NotNil(t, graph.ctx)
		assert.NotNil(t, graph.cancel)
		assert.Equal(t, "graph", graph.Name)
		assert.NotNil(t, graph.Nodes)
		assert.NotNil(t, graph.Supervisors)
	})
}

func TestGraph_AddSupervisor(t *testing.T) {
	t.Run("adds a supervisor to the graph", func(t *testing.T) {
		ctx := context.Background()
		graph := NewGraph(ctx, "graph", &Config{})

		parentSupervisorName := "parent supervisor"
		childSupervisorName := "child supervisor"
		supervisor1 := NewSupervisor(ctx, "parent supervisor", &graphSupervisorTestHandler{})
		supervisor2 := NewSupervisor(ctx, "child supervisor", &graphSupervisorTestHandler{})

		graph.AddSupervisor(nil, supervisor1)
		graph.AddSupervisor(supervisor1, supervisor2)

		assert.Contains(t, graph.Supervisors, parentSupervisorName)
		assert.Contains(t, graph.Supervisors, childSupervisorName)
		assert.Equal(t, parentSupervisorName, supervisor1.Name)
		assert.Equal(t, childSupervisorName, supervisor2.Name)
	})
}

func TestGraph_AddNode(t *testing.T) {
	t.Run("adds a node to the graph and assigns it to a supervisor", func(t *testing.T) {
		ctx := context.Background()
		graph := NewGraph(ctx, "graph", &Config{})

		supervisorName := "Test Supervisor"
		nodeName := "Test Node"

		supervisor := NewSupervisor(ctx, supervisorName, &graphSupervisorTestHandler{})
		graph.AddSupervisor(nil, supervisor)

		handler := &graphTestHandler{}
		node := NewNode[graphTestMessage, graphTestMessage](ctx, nodeName, handler)
		graph.AddNode(node)

		supervisor.AddNode(node)

		assert.Contains(t, graph.Nodes, nodeName)
		assert.Equal(t, nodeName, node.Name)
		assert.Equal(t, handler, node.Handler)
		assert.Contains(t, supervisor.Nodes, nodeName)
		assert.Equal(t, node, supervisor.Nodes[nodeName])
	})
}

func TestGraph_AddEdge(t *testing.T) {
	t.Run("adds an edge between two nodes", func(t *testing.T) {
		ctx := context.Background()

		supervisorName := "TestSupervisor"
		nodeName1 := "Node1"
		nodeName2 := "Node2"

		handler := &graphTestHandler{}

		supervisor := NewSupervisor(ctx, supervisorName, &graphSupervisorTestHandler{})
		node1 := NewNode[graphTestMessage, graphTestMessage](ctx, nodeName1, handler)
		node2 := NewNode[graphTestMessage, graphTestMessage](ctx, nodeName2, handler)
		edge := NewEdge("test edge", node1, node2, 10, 1)

		graph := NewGraph(ctx, "graph", &Config{}).
			AddSupervisor(nil, supervisor, node1, node2).
			AddNode(node1).
			AddNode(node2)

		graph.AddEdge(edge)

		// Verify channels are set up correctly
		assert.Equal(t, node1.Out.Name, "test edge")
		assert.Contains(t, node2.In, "test edge")

		// Verify the same channel is being used
		outEdge := node1.Out.GoChans["test edge"]
		inEdge := node2.In["test edge"]
		assert.Equal(t, outEdge, inEdge)

		// Verify the buffer size
		assert.Equal(t, 10, cap(outEdge.GetChannel()))
	})
}

func TestGraph_SimpleChannel(t *testing.T) {
	// Create a buffered channel
	inChannel := make(chan *Envelope[graphTestMessage], 10)

	// Create a test message
	message := graphTestMessage{Content: "test"}
	envelope := &Envelope[graphTestMessage]{Message: message, NumRetries: 3}

	// Send the envelope
	inChannel <- envelope

	// Receive the envelope
	receivedEnvelope := <-inChannel

	// Assert that the sent and received envelopes are the same
	assert.Equal(t, envelope, receivedEnvelope)
}

func TestGraph_Start(t *testing.T) {
	t.Run("starts all supervisors in the graph", func(t *testing.T) {
		ctx := context.Background()
		handler := &graphTestHandler{}

		supervisor := NewSupervisor(ctx, "TestSupervisor", &graphSupervisorTestHandler{})
		node1 := NewNode[graphTestMessage, graphTestMessage](ctx, "Node1", handler)
		node2 := NewNode[graphTestMessage, graphTestMessage](ctx, "Node2", handler)
		edge := NewEdge("test edge", node1, node2, 10, 1)

		graph := NewGraph(ctx, "graph", &Config{}).
			AddSupervisor(nil, supervisor, node1, node2).
			AddNode(node1).
			AddNode(node2).
			AddEdge(edge)

		// Start the graph
		assert.NotPanics(t, func() {
			graph.Start()
		})
	})
}
