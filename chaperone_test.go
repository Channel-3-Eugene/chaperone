package chaperone

import (
	"context"
	"crypto/rand"
	"errors"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testMessage struct {
	Content string
}

type testHandler struct{}

func (h *testHandler) Handle(msg *testMessage) (string, error) {
	if msg.Content == "error" {
		return "", NewEvent[testMessage](ErrorLevelError, errors.New("test error"), nil)
	}
	return "outChannel", nil
}

func randomString(n int) (string, error) {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		num, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			return "", err
		}
		b[i] = letters[num.Int64()]
	}
	return string(b), nil
}

func TestEndToEnd(t *testing.T) {
	// Step 1: Create the graph
	ctx := context.Background()
	graph := NewGraph[testMessage](ctx)

	// Step 2: Add supervisors to the graph
	supervisorName := "TestSupervisor"
	graph.AddSupervisor(supervisorName)
	supervisor := graph.Supervisors[supervisorName]

	// Step 3: Add nodes to the graph and assign them to the supervisor
	handler := &testHandler{}
	retryLimit := 3

	nodeName1 := "Node1"
	// graph.AddNode(supervisor, nodeName1, handler, retryLimit)
	graph.AddNode(supervisor, nodeName1, handler, retryLimit)
	node1 := graph.Nodes[nodeName1]

	nodeName2 := "Node2"
	graph.AddNode(supervisor, nodeName2, handler, retryLimit)
	node2 := graph.Nodes[nodeName2]

	// Step 4: Set up input and output channels for the nodes
	inCh1 := NewChannel[testMessage](10, false)
	node1.inputChans["input"] = inCh1

	graph.AddEdge(nodeName1, "outChannel", nodeName2, "inChannel", 10)

	outCh2 := NewChannel[testMessage](10, false)
	node2.outputChans["outChannel"] = OutMux[testMessage]{outChans: []chan *Envelope[testMessage]{outCh2}}

	// Step 5: Start the nodes
	node1.AddWorkers(1, "worker1", handler)
	node1.Start()

	node2.AddWorkers(1, "worker2", handler)
	node2.Start()

	// Step 6: Send random messages to the input channel of node 1
	msgContent, err := randomString(10)
	assert.NoError(t, err)
	msg := &testMessage{Content: msgContent}
	env := &Envelope[testMessage]{message: msg, numRetries: 3}
	inCh1 <- env

	// Step 7: Verify that the messages sent to node1 exits node2
	select {
	case received := <-outCh2:
		assert.Equal(t, msg, received.message, "Expected to receive the same message in node2 input channel")
	case <-time.After(1 * time.Millisecond):
		t.Error("Expected message in node2 output channel")
	}

	// Step 8: Send a message that will cause an error
	msgError := &testMessage{Content: "error"}
	envError := &Envelope[testMessage]{message: msgError, numRetries: 3}
	inCh1 <- envError

	// Verify that the error is handled and the message is sent to the event channel
	select {
	case ev := <-supervisor.events:
		assert.Equal(t, ErrorLevelError, ev.Level)
		assert.Contains(t, ev.Event.Error(), "test error")
		assert.Equal(t, msgError, ev.Message)
	case <-time.After(1 * time.Millisecond):
		t.Error("Timeout waiting for error event")
	}

	// Clean up by stopping the nodes
	node1.Stop()
	node2.Stop()
}
