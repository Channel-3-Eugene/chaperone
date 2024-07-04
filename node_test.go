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
}

type nodeTestHandler struct{}

func (h *nodeTestHandler) Handle(msg *nodeTestMessage) (string, error) {
	if msg.Content == "error" {
		return "", errors.New("test error")
	}
	return "outMux", nil
}

func TestNode_NewNode(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	handler := &nodeTestHandler{}

	node := newNode(ctx, cancel, "testNode", handler)

	assert.NotNil(t, node)
	assert.Equal(t, "testNode", node.Name)
	assert.Equal(t, handler, node.handler)
	assert.NotNil(t, node.devNull)
	assert.NotNil(t, node.eventChan) // Changed to assert not nil
}

func TestNode_AddWorkers(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	handler := &nodeTestHandler{}
	node := newNode(ctx, cancel, "testNode", handler)

	inCh := make(chan *Envelope[nodeTestMessage], 1)
	node.AddInputChannel("input", inCh)
	node.AddWorkers("input", 2, "worker")

	assert.Len(t, node.workerPool["input"], 2)
	assert.Equal(t, "worker-1", node.workerPool["input"][0].name)
	assert.Equal(t, "worker-2", node.workerPool["input"][1].name)
	assert.Equal(t, handler, node.workerPool["input"][0].handler)
	assert.Equal(t, handler, node.workerPool["input"][1].handler)
}

func TestNode_StartStop(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	handler := &nodeTestHandler{}
	node := newNode(ctx, cancel, "testNode", handler)

	inCh := make(chan *Envelope[nodeTestMessage], 1)
	node.AddInputChannel("input", inCh)
	node.AddWorkers("input", 1, "worker")
	node.Start()

	// Give some time for the worker to start
	time.Sleep(100 * time.Millisecond)

	// Check if worker is running
	assert.NotNil(t, node.workerPool["input"][0].ctx.Done())

	// Stop the node
	node.Stop()

	// Give some time for the worker to stop
	time.Sleep(100 * time.Millisecond)

	// Check if context is done
	assert.NotNil(t, node.workerPool["input"][0].ctx.Err())
	assert.Equal(t, context.Canceled, node.workerPool["input"][0].ctx.Err())
}

func TestNode_WorkerHandlesMessage(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	handler := &nodeTestHandler{}
	node := newNode(ctx, cancel, "testNode", handler)

	inCh := make(chan *Envelope[nodeTestMessage], 1)
	node.AddInputChannel("input", inCh)
	outCh := make(chan *Envelope[nodeTestMessage], 1)
	node.AddOutputChannel("outMux", "outChan", outCh)

	node.AddWorkers("input", 1, "worker")
	node.Start()

	// Send a message to the input channel
	msg := &nodeTestMessage{Content: "test"}
	env := &Envelope[nodeTestMessage]{message: msg, numRetries: 3}
	inCh <- env

	// Give some time for the worker to process the message
	time.Sleep(100 * time.Millisecond)

	// Check if the message was sent to the output channel
	received := <-outCh
	assert.Equal(t, msg, received.message)
}

func TestNode_WorkerHandlesError(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	handler := &nodeTestHandler{}
	node := newNode(ctx, cancel, "testNode", handler)

	inCh := make(chan *Envelope[nodeTestMessage], 1)
	node.AddInputChannel("input", inCh)
	node.eventChan = make(chan *Event[nodeTestMessage], 1)

	node.AddWorkers("input", 1, "worker")
	node.Start()

	// Send a message that will cause an error
	msg := &nodeTestMessage{Content: "error"}
	env := &Envelope[nodeTestMessage]{message: msg, numRetries: 3}
	inCh <- env

	// Give some time for the worker to process the message
	time.Sleep(100 * time.Millisecond)

	// Check if an event was sent to the event channel
	ev := <-node.eventChan
	assert.Equal(t, ErrorLevelError, ev.Level)
	assert.Equal(t, "test error", ev.Event.Error())
	assert.Equal(t, msg, ev.Message)
}
