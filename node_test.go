package chaperone

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type nodeTestMessage struct {
	Content string
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
		err := errors.New("test error")
		return nil, err
	}
	return env, nil
}

func (h *nodeTestHandler) Stop() {}

type nodeLoopbackTestHandler struct{}

func (h *nodeLoopbackTestHandler) Start(context.Context) error {
	return nil
}

func (h *nodeLoopbackTestHandler) Handle(_ context.Context, env Message) (Message, error) {
	if env.String() == "error" {
		return nil, errors.New("test error")
	}
	return env, nil
}

func (h *nodeLoopbackTestHandler) Stop() {}

func TestNode_NewNode(t *testing.T) {
	handler := &nodeTestHandler{}
	lbHandler := &nodeLoopbackTestHandler{}

	node := NewNode[nodeTestMessage, nodeTestMessage]("testNode", handler, lbHandler)

	assert.NotNil(t, node)
	assert.Equal(t, "testNode", node.Name())
	assert.Equal(t, handler, node.Handler)
	assert.Equal(t, lbHandler, node.LoopbackHandler)
	assert.Nil(t, node.Events)
}

func TestNode_AddWorkers(t *testing.T) {
	handler := &nodeTestHandler{}
	lbHandler := &nodeLoopbackTestHandler{}
	node := NewNode[nodeTestMessage, nodeTestMessage]("testNode", handler, lbHandler)

	inEdge := NewEdge("test", nil, node, 10, 2)

	assert.Len(t, node.WorkerPool[inEdge.Name()], 2)
	// test-worker-1 is the loopback worker
	assert.Equal(t, "testNode-loopback-1", node.WorkerPool["loopback"][0].name)
	assert.Equal(t, "test-worker-2", node.WorkerPool["test"][0].name)
	assert.Equal(t, "test-worker-3", node.WorkerPool["test"][1].name)
	assert.Equal(t, lbHandler, node.WorkerPool["loopback"][0].handler)
	assert.Equal(t, handler, node.WorkerPool["test"][0].handler)
	assert.Equal(t, handler, node.WorkerPool["test"][1].handler)
}

func TestNode_StartStop(t *testing.T) {
	handler := &nodeTestHandler{}
	lbHandler := &nodeLoopbackTestHandler{}
	node := NewNode[nodeTestMessage, nodeTestMessage]("testNode", handler, lbHandler)

	inEdge := NewEdge("test", nil, node, 10, 1)
	node.AddInput(inEdge)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	fmt.Printf("Test context address: %p\n", ctx)
	node.Start(ctx)

	// Give some time for the worker to start
	time.Sleep(1 * time.Millisecond)

	// Check if worker is running
	assert.Greater(t, node.RunningWorkerCount(), 0, "at least one worker should be running")

	// Stop the node
	evt := NewEvent(ErrorLevelError, errors.New("test error"), nil)
	node.Stop(evt)

	// Give some time for the worker to stop
	time.Sleep(50 * time.Millisecond)

	// Check if context is done
	assert.Equal(t, 0, node.RunningWorkerCount(), "all workers should have stopped")
}

func TestNode_WorkerHandlesMessage(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode[nodeTestMessage, nodeTestMessage]("testNode", handler, nil)

	inEdge := NewEdge("inEdge", nil, node, 10, 1)
	NewEdge("outEdge", node, nil, 10, 1)

	node.AddWorkers(inEdge, 1, "worker", handler)
	node.Start(context.Background())

	// Send a message to the input channel
	msg := nodeTestMessage{Content: "TestNode_WorkerHandlesMessage"}
	env := &Envelope[nodeTestMessage]{Message: msg, NumRetries: 3}
	inEdge.Send(env)

	// Give some time for the worker to process the message
	time.Sleep(10 * time.Millisecond)

	// Check if the message was sent to the output channel
	received := <-node.Out.GoChans["outEdge"].GetChannel()
	assert.Equal(t, msg.String(), received.String())
}

func TestNode_WorkerHandlesError(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode[nodeTestMessage, nodeTestMessage]("testNode", handler, nil)

	inEdge := NewEdge("test input", nil, node, 10, 1)
	node.AddInput(inEdge)

	node.Events = NewEdge("test events", nil, nil, 10, 1)

	node.AddWorkers(inEdge, 1, "worker", handler)
	node.Start(context.Background())

	// Send a message that will cause an error
	msg := nodeTestMessage{Content: "error"}
	env := &Envelope[nodeTestMessage]{Message: msg, NumRetries: 3}
	inEdge.GetChannel() <- env

	// Give some time for the worker to process the message
	time.Sleep(10 * time.Millisecond)

	// Check if an event was sent to the event channel
	evt := <-node.Events.GetChannel()
	ev, ok := evt.(*Event)
	assert.True(t, ok)
	assert.Equal(t, ErrorLevelError, ev.Level())
	assert.Equal(t, "[ERROR] test error", ev.Error())
}

func TestNode_RestartWorkers(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode[nodeTestMessage, nodeTestMessage]("testNode", handler, nil)

	inEdge := NewEdge("input", nil, node, 10, 2)

	node.Start(context.Background())

	// Give some time for the workers to start
	time.Sleep(10 * time.Millisecond)

	// Verify initial workers are running
	assert.Len(t, node.WorkerPool[inEdge.Name()], 2)

	// Restart workers
	node.RestartWorkers(context.Background())

	// Give some time for the workers to restart
	time.Sleep(10 * time.Millisecond)

	// Verify workers have been restarted
	assert.Len(t, node.WorkerPool[inEdge.Name()], 2)
	assert.Equal(t, "input-worker-1", node.WorkerPool[inEdge.Name()][0].name)
	assert.Equal(t, "input-worker-2", node.WorkerPool[inEdge.Name()][1].name)
}

func TestNode_NoAdditionalInputChannels(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode[nodeTestMessage, nodeTestMessage]("testNode", handler, handler)

	// Start the node without additional input channels
	node.Start(context.Background())

	// Give some time for the worker to start
	time.Sleep(10 * time.Millisecond)

	// Ensure no additional workers were started
	assert.Len(t, node.WorkerPool, 1) // Only loopback worker should exist
	assert.Equal(t, "testNode-loopback-1", node.WorkerPool["loopback"][0].name)
}
