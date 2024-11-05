package chaperone

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

type edgeTestMessage struct {
	Content string
}

func (m edgeTestMessage) String() string {
	return m.Content
}

type edgeTestHandler struct {
	outChannelName string
}

func (h *edgeTestHandler) Start(context.Context) error {
	return nil
}

func (h *edgeTestHandler) Handle(ctx context.Context, env Message) (Message, error) {
	if env.String() == "error" {
		return nil, errors.New("test error")
	}

	return env, nil
}

func (h *edgeTestHandler) Stop() {}

func TestEdge_NewEdge(t *testing.T) {
	node1 := NewNode[edgeTestMessage, edgeTestMessage]("node1", &edgeTestHandler{outChannelName: "output"}, nil)
	node2 := NewNode[edgeTestMessage, edgeTestMessage]("node2", &edgeTestHandler{outChannelName: "output"}, nil)

	edgeName := "edge1"
	edge := NewEdge(edgeName, node1, node2, 10, 1)
	assert.NotNil(t, edge)
	assert.NotNil(t, edge.GetChannel())
	assert.Equal(t, 10, cap(edge.GetChannel()))

	assert.Equal(t, "node1:output", node1.Out.Name)
	assert.Contains(t, node2.In, edgeName)

	outChannel := node1.Out.GoChans["node2:input"]
	inChannel := node2.In["node2:input"]
	assert.Equal(t, outChannel, inChannel)
}

func TestEdge_Close(t *testing.T) {
	node1 := NewNode[edgeTestMessage, edgeTestMessage]("node1", &edgeTestHandler{outChannelName: "output"}, nil)
	node2 := NewNode[edgeTestMessage, edgeTestMessage]("node2", &edgeTestHandler{outChannelName: "output"}, nil)

	edge := NewEdge("edge1", node1, node2, 10, 1)
	assert.NotNil(t, edge)
	assert.NotNil(t, edge.GetChannel())

	var wg sync.WaitGroup
	numRoutines := 10

	// Spawn multiple goroutines trying to close the channel concurrently
	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			edge.Close() // Calling Close concurrently
		}()
	}

	// Wait for all goroutines to finish
	wg.Wait()

	// Check that the channel is actually closed (attempting to send should panic)
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("panic occurred: %v", r)
		}
	}()

	// Assert that the channel is closed
	_, ok := <-edge.GetChannel()
	assert.False(t, ok, "channel should be closed")
}
