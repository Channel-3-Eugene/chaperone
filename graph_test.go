package chaperone

import (
	"context"
	"errors"
	"testing"
	"time"

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

func (h *graphTestHandler) Stop() {}

type graphSupervisorTestHandler struct{}

func (h *graphSupervisorTestHandler) Start(context.Context) error {
	return nil
}

func (h *graphSupervisorTestHandler) Handle(_ context.Context, evt Message) error {
	return nil
}

func (h *graphSupervisorTestHandler) Stop() {}

func TestGraph_NewGraph(t *testing.T) {
	t.Run("creates a new graph with the given context", func(t *testing.T) {
		graph := NewGraph("graph", &Config{})

		assert.NotNil(t, graph)
		assert.Equal(t, "graph", graph.Name)
		assert.NotNil(t, graph.Nodes)
		assert.NotNil(t, graph.Supervisors)
	})
}

func TestGraph_AddSupervisor(t *testing.T) {
	t.Run("adds a supervisor to the graph", func(t *testing.T) {
		graph := NewGraph("graph", &Config{})

		parentSupervisorName := "parent supervisor"
		childSupervisorName := "child supervisor"
		supervisor1 := NewSupervisor("parent supervisor", &graphSupervisorTestHandler{})
		supervisor2 := NewSupervisor("child supervisor", &graphSupervisorTestHandler{})

		graph.AddSupervisor(nil, supervisor1)
		graph.AddSupervisor(supervisor1, supervisor2)

		assert.Contains(t, graph.Supervisors, parentSupervisorName)
		assert.Contains(t, supervisor1.Supervisors, childSupervisorName)
		assert.Equal(t, parentSupervisorName, supervisor1.Name())
		assert.Equal(t, childSupervisorName, supervisor2.Name())
	})
}

func TestGraph_AddNode(t *testing.T) {
	t.Run("adds a node to the graph and assigns it to a supervisor", func(t *testing.T) {
		graph := NewGraph("graph", &Config{})

		supervisorName := "Test Supervisor"
		nodeName := "Test Node"

		supervisor := NewSupervisor(supervisorName, &graphSupervisorTestHandler{})
		graph.AddSupervisor(nil, supervisor)

		handler := &graphTestHandler{}
		node := NewNode[graphTestMessage, graphTestMessage](nodeName, handler, nil)
		graph.AddNode(supervisor, node)

		assert.Contains(t, graph.Nodes, nodeName)
		assert.Equal(t, nodeName, node.Name())
		assert.Equal(t, handler, node.Handler)
		assert.Contains(t, supervisor.Nodes, nodeName)
		assert.Equal(t, node, supervisor.Nodes[nodeName])
		assert.Len(t, node.WorkerPool["loopback"], 0)
	})
}

func TestGraph_AddEdge(t *testing.T) {
	t.Run("adds an edge between two nodes", func(t *testing.T) {
		supervisorName := "TestSupervisor"
		nodeName1 := "Node1"
		nodeName2 := "Node2"

		handler := &graphTestHandler{}

		supervisor := NewSupervisor(supervisorName, &graphSupervisorTestHandler{})
		node1 := NewNode[graphTestMessage, graphTestMessage](nodeName1, handler, nil)
		node2 := NewNode[graphTestMessage, graphTestMessage](nodeName2, handler, nil)
		edge := NewEdge("test edge", node1, node2, 10, 1)

		graph := NewGraph("graph", &Config{}).
			AddSupervisor(nil, supervisor).
			AddNode(supervisor, node1).
			AddNode(supervisor, node2)

		graph.AddEdge(edge)

		// Verify channels are set up correctly
		assert.Equal(t, "Node1:output", node1.Out.Name)
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

		supervisor := NewSupervisor("TestSupervisor", &graphSupervisorTestHandler{})
		node1 := NewNode[graphTestMessage, graphTestMessage]("Node1", handler, nil)
		node2 := NewNode[graphTestMessage, graphTestMessage]("Node2", handler, nil)
		edge := NewEdge("test edge", node1, node2, 10, 1)

		graph := NewGraph("graph", &Config{}).
			AddSupervisor(nil, supervisor).
			AddNode(supervisor, node1).
			AddNode(supervisor, node2).
			AddEdge(edge)

		// Start the graph
		assert.NotPanics(t, func() {
			graph.Start(ctx)
		})
	})
}

func TestGraph_Metrics(t *testing.T) {
	t.Run("returns metrics for all nodes", func(t *testing.T) {
		handler := &graphTestHandler{}
		graph := NewGraph("graph", &Config{})

		supervisor := NewSupervisor("TestSupervisor", &graphSupervisorTestHandler{})
		node1 := NewNode[graphTestMessage, graphTestMessage]("Node1", handler, nil)
		node2 := NewNode[graphTestMessage, graphTestMessage]("Node2", handler, nil)
		node3 := NewNode[graphTestMessage, graphTestMessage]("Node3", handler, nil)

		graph.AddSupervisor(nil, supervisor).
			AddNode(supervisor, node1).
			AddNode(supervisor, node2).
			AddNode(supervisor, node3)

		graph.Start(context.Background())

		// Send some messages to simulate traffic
		for i := 0; i < 10; i++ {
			msg := graphTestMessage{Content: "test"}
			env := &Envelope[graphTestMessage]{Message: msg, NumRetries: 3}
			node1.In["loopback"].Send(env)
		}

		// Wait for metrics to update
		time.Sleep(100 * time.Millisecond)

		// Get metrics for all nodes
		metrics := graph.Metrics(nil)

		// Verify that metrics are returned for all nodes
		assert.Len(t, metrics, 3, "should return metrics for all 3 nodes")
		for _, m := range metrics {
			assert.Contains(t, []string{"Node1", "Node2", "Node3"}, m.NodeName)
		}
	})

	t.Run("returns metrics for a subset of nodes", func(t *testing.T) {
		handler := &graphTestHandler{}
		graph := NewGraph("graph", &Config{})

		supervisor := NewSupervisor("TestSupervisor", &graphSupervisorTestHandler{})
		node1 := NewNode[graphTestMessage, graphTestMessage]("Node1", handler, nil)
		node2 := NewNode[graphTestMessage, graphTestMessage]("Node2", handler, nil)
		node3 := NewNode[graphTestMessage, graphTestMessage]("Node3", handler, nil)

		graph.AddSupervisor(nil, supervisor).
			AddNode(supervisor, node1).
			AddNode(supervisor, node2).
			AddNode(supervisor, node3)

		graph.Start(context.Background())

		// Send some messages to simulate traffic
		for i := 0; i < 10; i++ {
			msg := graphTestMessage{Content: "test"}
			env := &Envelope[graphTestMessage]{Message: msg, NumRetries: 3}
			node1.In["loopback"].Send(env)
		}

		// Wait for metrics to update
		time.Sleep(110 * time.Millisecond)

		// Get metrics for Node1 and Node3
		metrics := graph.Metrics([]string{"Node1", "Node3"})

		// Verify that metrics are returned only for Node1 and Node3
		assert.Len(t, metrics, 2, "should return metrics for the specified nodes")
		for _, m := range metrics {
			assert.Contains(t, []string{"Node1", "Node3"}, m.NodeName)
		}
	})
}
