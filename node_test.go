package chaperone

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type nodeTestMessage struct {
	Content string
	Mux     string
}

func (m nodeTestMessage) String() string {
	return m.Content
}

type nodeTestHandler struct{}

func (h *nodeTestHandler) Start(context.Context) error {
	return nil
}

func (h *nodeTestHandler) Handle(_ context.Context, env Message) (Message, error) {
	if env.String() == "error" {
		return nil, errors.New("test error")
	}
	return env, nil
}

func TestNode_NewNode(t *testing.T) {
	handler := &nodeTestHandler{}

	node := NewNode[nodeTestMessage, nodeTestMessage](context.Background(), "testNode", handler)

	assert.NotNil(t, node)
	assert.Equal(t, "testNode", node.Name)
	assert.Equal(t, handler, node.Handler)
	assert.NotNil(t, node.Events)
}

func TestNode_AddWorkers(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode[nodeTestMessage, nodeTestMessage](context.Background(), "testNode", handler)

	inEdge := NewEdge("test input", nil, node, 10, 1)
	node.AddInput("input", inEdge)
	node.AddWorkers(inEdge, 2, "worker")

	assert.Len(t, node.WorkerPool["input"], 2)
	assert.Equal(t, "input-worker-1", node.WorkerPool["input"][0].name)
	assert.Equal(t, "input-worker-2", node.WorkerPool["input"][1].name)
	assert.Equal(t, handler, node.WorkerPool["input"][0].handler)
	assert.Equal(t, handler, node.WorkerPool["input"][1].handler)
}

func TestNode_StartStop(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode[nodeTestMessage, nodeTestMessage](context.Background(), "testNode", handler)

	inEdge := NewEdge("test input", nil, node, 10, 1)
	node.AddInput("input", inEdge)
	node.AddWorkers(inEdge, 1, "worker")
	node.Start()

	// Give some time for the worker to start
	time.Sleep(100 * time.Millisecond)

	// Check if worker is running
	assert.NotNil(t, node.WorkerPool["input"][0].ctx.Done())

	// Stop the node
	evt := NewEvent(ErrorLevelError, errors.New("test error"), nil)
	node.Stop(evt)

	// Give some time for the worker to stop
	time.Sleep(100 * time.Millisecond)

	// Check if context is done
	assert.NotNil(t, node.WorkerPool["input"][0].ctx.Err())
	assert.Equal(t, context.Canceled, node.WorkerPool["input"][0].ctx.Err())
}

func TestNode_WorkerHandlesMessage(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode[nodeTestMessage, nodeTestMessage](context.Background(), "testNode", handler)

	inEdge := NewEdge("test input", nil, node, 10, 1)
	node.AddInput("input", inEdge)
	outEdge := NewEdge("test output", node, nil, 10, 1)
	node.AddOutput(outEdge)

	node.AddWorkers(inEdge, 1, "worker")
	node.Start()

	// Send a message to the input channel
	msg := nodeTestMessage{Content: "test", Mux: "outMux"}
	env := &Envelope[nodeTestMessage]{Message: msg, NumRetries: 3}
	inEdge.GetChannel() <- env

	// Give some time for the worker to process the message
	time.Sleep(100 * time.Millisecond)

	// Check if the message was sent to the output channel
	received := <-outEdge.GetChannel()
	assert.Equal(t, msg.String(), received.String())
}

func TestNode_WorkerHandlesError(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode[nodeTestMessage, nodeTestMessage](context.Background(), "testNode", handler)

	inEdge := NewEdge("test input", nil, node, 10, 1)
	node.AddInput("input", inEdge)
	node.Events = NewEdge("test events", nil, node, 10, 1)

	node.AddWorkers(inEdge, 1, "worker")
	node.Start()

	// Send a message that will cause an error
	msg := nodeTestMessage{Content: "error"}
	env := &Envelope[nodeTestMessage]{Message: msg, NumRetries: 3}
	inEdge.GetChannel() <- env

	// Give some time for the worker to process the message
	time.Sleep(100 * time.Millisecond)

	// Check if an event was sent to the event channel
	evt := <-node.Events.GetChannel()
	ev, ok := evt.(*Event)
	assert.True(t, ok)
	assert.Equal(t, ErrorLevelError, ev.Level())
	assert.Equal(t, "test error", ev.Error())
	assert.Equal(t, msg.String(), ev.String())
}

func TestNode_RestartWorkers(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode[nodeTestMessage, nodeTestMessage](context.Background(), "testNode", handler)

	inEdge := NewEdge("test input", nil, node, 10, 1)
	node.AddInput("input", inEdge)
	node.AddWorkers(inEdge, 2, "worker")

	node.Start()

	// Give some time for the workers to start
	time.Sleep(100 * time.Millisecond)

	// Verify initial workers are running
	assert.Len(t, node.WorkerPool["input"], 2)

	// Restart workers
	node.RestartWorkers()

	// Give some time for the workers to restart
	time.Sleep(100 * time.Millisecond)

	// Verify workers have been restarted
	assert.Len(t, node.WorkerPool["input"], 2)
	assert.Equal(t, "input-worker-3", node.WorkerPool["input"][0].name)
	assert.Equal(t, "input-worker-4", node.WorkerPool["input"][1].name)
}
