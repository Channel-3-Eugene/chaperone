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

func (h *nodeTestHandler) Handle(_ context.Context, env *Envelope[nodeTestMessage]) error {
	if env.Message.Content == "error" {
		return errors.New("test error")
	}
	env.OutChan = env.CurrentNode.OutputChans[env.Message.Mux]
	return nil
}

func TestNode_NewNode(t *testing.T) {
	handler := &nodeTestHandler{}

	node := NewNode(context.Background(), "testNode", handler)

	assert.NotNil(t, node)
	assert.Equal(t, "testNode", node.Name)
	assert.Equal(t, handler, node.Handler)
	assert.NotNil(t, node.EventChan) // Changed to assert not nil
}

func TestNode_AddWorkers(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode(context.Background(), "testNode", handler)

	inCh := make(chan *Envelope[nodeTestMessage], 1)
	node.AddInputChannel("input", inCh)
	node.AddWorkers("input", 2, "worker")

	assert.Len(t, node.WorkerPool["input"], 2)
	assert.Equal(t, "worker-1", node.WorkerPool["input"][0].name)
	assert.Equal(t, "worker-2", node.WorkerPool["input"][1].name)
	assert.Equal(t, handler, node.WorkerPool["input"][0].handler)
	assert.Equal(t, handler, node.WorkerPool["input"][1].handler)
}

func TestNode_StartStop(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode(context.Background(), "testNode", handler)

	inCh := make(chan *Envelope[nodeTestMessage], 1)
	node.AddInputChannel("input", inCh)
	node.AddWorkers("input", 1, "worker")
	node.Start()

	// Give some time for the worker to start
	time.Sleep(100 * time.Millisecond)

	// Check if worker is running
	assert.NotNil(t, node.WorkerPool["input"][0].ctx.Done())

	// Stop the node
	node.Stop()

	// Give some time for the worker to stop
	time.Sleep(100 * time.Millisecond)

	// Check if context is done
	assert.NotNil(t, node.WorkerPool["input"][0].ctx.Err())
	assert.Equal(t, context.Canceled, node.WorkerPool["input"][0].ctx.Err())
}

func TestNode_WorkerHandlesMessage(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode(context.Background(), "testNode", handler)

	inCh := make(chan *Envelope[nodeTestMessage], 1)
	node.AddInputChannel("input", inCh)
	outCh := make(chan *Envelope[nodeTestMessage], 1)
	node.AddOutputChannel("outMux", "outChan", outCh)

	node.AddWorkers("input", 1, "worker")
	node.Start()

	// Send a message to the input channel
	msg := nodeTestMessage{Content: "test", Mux: "outMux"}
	env := &Envelope[nodeTestMessage]{Message: msg, NumRetries: 3}
	inCh <- env

	// Give some time for the worker to process the message
	time.Sleep(100 * time.Millisecond)

	// Check if the message was sent to the output channel
	received := <-outCh
	assert.Equal(t, msg, received.Message)
}

func TestNode_WorkerHandlesError(t *testing.T) {
	handler := &nodeTestHandler{}
	node := NewNode(context.Background(), "testNode", handler)

	inCh := make(chan *Envelope[nodeTestMessage], 1)
	node.AddInputChannel("input", inCh)
	node.EventChan = make(chan *Event[nodeTestMessage], 1)

	node.AddWorkers("input", 1, "worker")
	node.Start()

	// Send a message that will cause an error
	msg := nodeTestMessage{Content: "error"}
	env := &Envelope[nodeTestMessage]{Message: msg, NumRetries: 3}
	inCh <- env

	// Give some time for the worker to process the message
	time.Sleep(100 * time.Millisecond)

	// Check if an event was sent to the event channel
	ev := <-node.EventChan
	assert.Equal(t, ErrorLevelError, ev.Level)
	assert.Equal(t, "test error", ev.Event.Error())
	assert.Equal(t, msg, ev.Envelope.Message)
}
