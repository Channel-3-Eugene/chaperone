package chaperone

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func (ti *TestItem) SetData(data string) {
	ti.data = data
}

type TestWorker[E *Envelope[I], I any] struct {
	id        int
	running   bool
	name      string
	handleErr error
}

func (w *TestWorker[E, I]) Handle(ctx context.Context, msg E) (E, Channel[E, I], error) {
	if w.handleErr != nil {
		return msg, nil, w.handleErr
	}

	return msg, nil, nil
}

func (w *TestWorker[E, I]) Start(ctx context.Context) {
	w.running = true
	println("Worker started")
}

func (w *TestWorker[E, I]) Stop() {
	w.running = false
	println("Worker stopped")
}

func (w *TestWorker[E, I]) Name() string {
	return w.name
}

func (w *TestWorker[E, I]) Status() string {
	if w.running {
		return "Running"
	}
	return "Idle"
}

type TestWorkerFactory[E *Envelope[I], I any] struct{}

func (f TestWorkerFactory[E, I]) CreateWorker(id int) WorkerInterface[E, I] {
	return &TestWorker[E, I]{
		id:   id,
		name: fmt.Sprintf("Worker%d", id),
	}
}

func TestNode_NewNode(t *testing.T) {
	factory := TestWorkerFactory[*Envelope[TestItem], TestItem]{}
	node := NewNode("1", "TestNode", 3, 5, factory)

	assert.Equal(t, "1", node.ID)
	assert.Equal(t, "TestNode", node.Name)
	assert.Len(t, node.workerPool, 3)
	assert.Equal(t, 5, node.RetryLimit)
}

func TestNode_Start(t *testing.T) {
	factory := TestWorkerFactory[*Envelope[TestItem], TestItem]{}
	node := NewNode("1", "TestNode", 3, 5, factory)

	ctx := context.Background()

	node.Start(ctx)

	assert.Len(t, node.workerPool, 3)
	for _, w := range node.workerPool {
		assert.Equal(t, "Running", w.Status())
	}

	node.Stop(ctx)

	for _, w := range node.workerPool {
		assert.Equal(t, "Idle", w.Status())
	}
}

func TestNode_MessageProcessing(t *testing.T) {
	factory := TestWorkerFactory[*Envelope[TestItem], TestItem]{}
	node := NewNode("1", "TestNode", 3, 5, factory)

	ctx := context.Background()

	inputChan := make(Channel[*Envelope[TestItem], TestItem])
	outputChan := make(Channel[*Envelope[TestItem], TestItem])

	node.InputChans[0] = inputChan
	node.OutputChans[0] = outputChan

	node.Start(ctx)

	env := NewEnvelope(TestItem{data: "test"}, 3)
	inputChan <- env

	select {
	case processedEnv := <-outputChan:
		assert.Equal(t, "test", processedEnv.Item().String())
	case <-time.After(5 * time.Millisecond):
		assert.Fail(t, "timeout waiting for message to be processed")
	}

	node.Stop(ctx)
}

func TestNode_RetryLimit(t *testing.T) {
	factory := TestWorkerFactory[*Envelope[TestItem], TestItem]{}
	node := NewNode("1", "TestNode", 3, 5, factory)

	worker := &TestWorker[*Envelope[TestItem], TestItem]{id: 0, handleErr: fmt.Errorf("error")}
	node.workerPool[0] = worker

	ctx := context.Background()

	inputChan := make(Channel[*Envelope[TestItem], TestItem])

	node.InputChans[0] = inputChan
	node.OutputChans[DEVNULL] = NewDevNullChannel[*Envelope[TestItem], TestItem](ctx)

	node.Start(ctx)

	msg := NewEnvelope(TestItem{data: "test"}, 2)
	inputChan <- msg

	select {
	case <-node.OutputChans[DEVNULL]:
		// Success case
	case <-time.After(1 * time.Second):
		assert.Fail(t, "timeout waiting for message to reach devnull")
	}

	node.Stop(ctx)
}

func TestNode_HandleWorkerError(t *testing.T) {
	factory := TestWorkerFactory[*Envelope[TestItem], TestItem]{}
	node := NewNode("1", "TestNode", 3, 2, factory)

	worker := &TestWorker[*Envelope[TestItem], TestItem]{id: 0, handleErr: fmt.Errorf("error")}
	node.workerPool[0] = worker

	ctx := context.Background()

	inputChan := make(Channel[*Envelope[TestItem], TestItem])

	node.InputChans[0] = inputChan
	node.OutputChans[DEVNULL] = NewDevNullChannel[*Envelope[TestItem], TestItem](ctx)

	node.Start(ctx)

	msg := NewEnvelope(TestItem{data: "test"}, 2)
	inputChan <- msg

	select {
	case <-node.OutputChans[DEVNULL]:
		// Success case
	case <-time.After(1 * time.Second):
		assert.Fail(t, "timeout waiting for message to reach devnull")
	}

	assert.Equal(t, 0, node.errorCount[0], "expected error count to be reset")
	assert.True(t, node.shortCircuit[0], "expected input to be short-circuited")

	node.Stop(ctx)
}
