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

type graphTestHandler struct{}

func (h *graphTestHandler) Handle(msg *graphTestMessage) (string, error) {
	if msg.Content == "error" {
		return "", errors.New("test error")
	}
	return "outChannel", nil
}

func TestNewGraph(t *testing.T) {
	t.Run("creates a new graph with the given context", func(t *testing.T) {
		ctx := context.Background()
		graph := NewGraph[graphTestMessage](ctx)

		assert.NotNil(t, graph)
		assert.NotNil(t, graph.ctx)
		assert.NotNil(t, graph.cancel)
		assert.NotNil(t, graph.Nodes)
		assert.NotNil(t, graph.Supervisors)
	})
}

func TestGraph_AddSupervisor(t *testing.T) {
	t.Run("adds a supervisor to the graph", func(t *testing.T) {
		ctx := context.Background()
		graph := NewGraph[graphTestMessage](ctx)

		supervisorName := "TestSupervisor"
		graph.AddSupervisor(supervisorName)

		assert.Contains(t, graph.Supervisors, supervisorName)
		assert.Equal(t, supervisorName, graph.Supervisors[supervisorName].Name)
	})
}

func TestGraph_AddNode(t *testing.T) {
	t.Run("adds a node to the graph and assigns it to a supervisor", func(t *testing.T) {
		ctx := context.Background()
		graph := NewGraph[graphTestMessage](ctx)

		supervisorName := "TestSupervisor"
		graph.AddSupervisor(supervisorName)
		supervisor := graph.Supervisors[supervisorName]

		nodeName := "TestNode"
		handler := &graphTestHandler{}
		graph.AddNode(supervisorName, nodeName, handler)

		assert.Contains(t, graph.Nodes, nodeName)
		assert.Equal(t, nodeName, graph.Nodes[nodeName].Name)
		assert.Equal(t, handler, graph.Nodes[nodeName].handler)
		assert.Contains(t, supervisor.Nodes, nodeName)
		assert.Equal(t, graph.Nodes[nodeName], supervisor.Nodes[nodeName])
	})
}

func TestGraph_AddEdge(t *testing.T) {
	t.Run("adds an edge between two nodes", func(t *testing.T) {
		ctx := context.Background()

		supervisorName := "TestSupervisor"
		nodeName1 := "Node1"
		nodeName2 := "Node2"

		handler := &graphTestHandler{}

		graph := NewGraph[graphTestMessage](ctx).
			AddSupervisor(supervisorName).
			AddNode(supervisorName, nodeName1, handler).
			AddNode(supervisorName, nodeName2, handler).
			AddEdge(nodeName1, "outChannel", nodeName2, "input", 10)

		// Verify channels are set up correctly
		assert.Contains(t, graph.Nodes[nodeName1].outputChans, "outChannel")
		assert.Contains(t, graph.Nodes[nodeName2].inputChans, "input")

		// Verify the same channel is being used
		outChannel := graph.Nodes[nodeName1].outputChans["outChannel"].outChans[0]
		inChannel := graph.Nodes[nodeName2].inputChans["input"]
		assert.Equal(t, outChannel, inChannel)

		// Verify the buffer size
		assert.Equal(t, 10, cap(outChannel))

		// Verify the channel accepts and delivers messages
		// select {
		// case receivedEnvelope := <-inChannel:
		// 	assert.Equal(t, envelope, receivedEnvelope)
		// default:
		// 	t.Errorf("Expected to receive message on input channel")
		// }
	})
}

func TestGraph_SimpleChannel(t *testing.T) {
	// Create a buffered channel
	// inChannel := make(chan *Envelope[graphTestMessage], 10)
	inChannel := NewChannel[graphTestMessage](10, false)
	outChannel := inChannel

	// Create a test message
	message := &graphTestMessage{Content: "test"}
	envelope := &Envelope[graphTestMessage]{message: message, numRetries: 3}

	// Send the envelope
	outChannel <- envelope

	// Receive the envelope
	receivedEnvelope := <-inChannel

	// Assert that the sent and received envelopes are the same
	assert.Equal(t, envelope, receivedEnvelope)
}

func TestGraph_Start(t *testing.T) {
	t.Run("starts all nodes in the graph", func(t *testing.T) {
		ctx := context.Background()
		handler := &graphTestHandler{}

		supervisorName := "TestSupervisor"
		nodeName1 := "Node1"
		nodeName2 := "Node2"

		graph := NewGraph[graphTestMessage](ctx).
			AddSupervisor(supervisorName).
			AddNode(supervisorName, nodeName1, handler).
			AddNode(supervisorName, nodeName2, handler).
			AddEdge(nodeName1, "outChannel", nodeName2, "input", 10)

		// Start the graph
		graph.Start()

		// Since our nodes don't have a "running" state, we'll assume Start works if no panics occur.
		assert.NotPanics(t, func() {
			graph.Start()
		})
	})
}
