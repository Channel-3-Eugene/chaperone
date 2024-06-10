package chaperone

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type SupervisorTestMessage struct {
	Data    string
	Payload int
}

type SupervisorTestWorker struct {
	id        int
	started   bool
	stopped   bool
	name      string
	status    string
	handleErr error
}

func (w *SupervisorTestWorker) Handle(ctx context.Context, msg *Envelope[SupervisorTestMessage]) (*Envelope[SupervisorTestMessage], Channel[*Envelope[SupervisorTestMessage], SupervisorTestMessage], error) {
	if w.handleErr != nil && msg.Item().Payload == -1 {
		return msg, nil, w.handleErr
	}
	newData := msg.Item().Data + "_handled"
	msg.SetItem(SupervisorTestMessage{Data: newData, Payload: msg.Item().Payload})
	return msg, nil, nil
}

func (w *SupervisorTestWorker) Start(ctx context.Context) {
	w.started = true
	w.status = "Running"
}

func (w *SupervisorTestWorker) Stop() {
	w.stopped = true
	w.status = "Idle"
}

func (w *SupervisorTestWorker) Name() string {
	return w.name
}

func (w *SupervisorTestWorker) Status() string {
	return w.status
}

type SupervisorTestWorkerFactory struct{}

func (f *SupervisorTestWorkerFactory) CreateWorker(id int) WorkerInterface[*Envelope[SupervisorTestMessage], SupervisorTestMessage] {
	return &SupervisorTestWorker{
		id:     id,
		name:   fmt.Sprintf("Worker%d", id),
		status: "Idle",
	}
}

func TestSupervisorHandling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	factory := &SupervisorTestWorkerFactory{}
	node := NewNode[*Envelope[SupervisorTestMessage], SupervisorTestMessage]("testNode", "Test Node", 1, 3, factory)
	errorBucket := make(Channel[*Envelope[SupervisorTestMessage], SupervisorTestMessage], 10)
	supervisor := NewSupervisor[*Envelope[SupervisorTestMessage], SupervisorTestMessage]("testSupervisor", &node, nil, errorBucket, 3)

	input := make(Channel[*Envelope[SupervisorTestMessage], SupervisorTestMessage], 10)
	output := make(Channel[*Envelope[SupervisorTestMessage], SupervisorTestMessage], 10)
	devnull := NewDevNullChannel[*Envelope[SupervisorTestMessage], SupervisorTestMessage](ctx)

	node.InputChans[0] = input
	node.OutputChans[0] = output
	node.OutputChans[DEVNULL] = devnull

	var wg sync.WaitGroup
	supervisor.Start(ctx)

	// Test valid message processing
	input <- NewEnvelope(SupervisorTestMessage{Payload: 42}, 3)

	select {
	case msg := <-output:
		assert.Equal(t, 42, msg.Item().Payload)
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "Message not processed in time")
	}

	// Test error message processing with retries
	input <- NewEnvelope(SupervisorTestMessage{Payload: -1}, 3)

	select {
	case msg := <-devnull:
		assert.Equal(t, -1, msg.Item().Payload)
	case <-time.After(100 * time.Millisecond):
		assert.Fail(t, "Error message not processed in time")
	}

	cancel()
	wg.Wait()
}
