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
	return "outChannel", nil
}

func TestNode_NewNode(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	handler := &nodeTestHandler{}

	node := newNode(ctx, cancel, "testNode", handler)

	assert.NotNil(t, node)
	assert.Equal(t, "testNode", node.Name)
	assert.Equal(t, handler, node.handler)
	assert.NotNil(t, node.devNull)
	assert.Nil(t, node.eventChan)
}

func TestNode_AddWorkers(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	handler := &nodeTestHandler{}
	node := newNode(ctx, cancel, "testNode", handler)

	node.AddWorkers(2, "worker", handler)

	assert.Len(t, node.workerPool, 2)
	assert.Equal(t, "worker-1", node.workerPool[0].name)
	assert.Equal(t, "worker-2", node.workerPool[1].name)
	assert.Equal(t, handler, node.workerPool[0].handler)
	assert.Equal(t, handler, node.workerPool[1].handler)
}

func TestNode_StartStop(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	handler := &nodeTestHandler{}
	node := newNode(ctx, cancel, "testNode", handler)

	node.AddWorkers(1, "worker", handler)
	node.Start()

	// Give some time for the worker to start
	time.Sleep(100 * time.Millisecond)

	// Check if worker is running
	assert.NotNil(t, node.workerPool[0].ctx.Done())

	// Stop the node
	node.Stop()

	// Give some time for the worker to stop
	time.Sleep(100 * time.Millisecond)

	// Check if context is done
	assert.NotNil(t, node.workerPool[0].ctx.Err())
	assert.Equal(t, context.Canceled, node.workerPool[0].ctx.Err())
}

func TestNode_WorkerHandlesMessage(t *testing.T) {
	ctx, cancel := context.WithCancelCause(context.Background())
	handler := &nodeTestHandler{}
	node := newNode(ctx, cancel, "testNode", handler)

	inCh := make(chan *Envelope[nodeTestMessage], 1)
	node.inputChans["input"] = inCh
	outCh := make(chan *Envelope[nodeTestMessage], 1)
	node.outputChans["outChannel"] = OutMux[nodeTestMessage]{outChans: []chan *Envelope[nodeTestMessage]{outCh}}

	node.AddWorkers(1, "worker", handler)
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
	node.inputChans["input"] = inCh
	node.eventChan = make(chan *Event[nodeTestMessage], 1)

	node.AddWorkers(1, "worker", handler)
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
