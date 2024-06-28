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
	// Create the graph
	ctx := context.Background()

	handler := &testHandler{}
	RetryLimit := 3
	SupervisorName := "supervisor1"
	Node1Name := "node1"
	Node2Name := "node2"

	graph := NewGraph[testMessage](ctx).
		AddSupervisor(SupervisorName).
		AddNode(SupervisorName, Node1Name, handler, RetryLimit).
		AddWorkers(Node1Name, 1, "worker1", handler).
		AddNode(SupervisorName, Node2Name, handler, RetryLimit).
		AddWorkers(Node2Name, 1, "worker2", handler).
		AddEdge("", "", Node1Name, "input", 10).
		AddEdge(Node1Name, "outChannel", Node2Name, "input", 10).
		AddEdge(Node2Name, "outChannel", "", "", 10).
		Start()

	// Set up input and output channels for the nodes
	inCh1 := graph.Nodes[Node1Name].inputChans["input"]
	outCh2 := graph.Nodes[Node2Name].outputChans["outChannel"].outChans[0]

	// Send random messages to the input channel of node 1
	msgContent, err := randomString(10)
	assert.NoError(t, err)
	msg := &testMessage{Content: msgContent}
	env := &Envelope[testMessage]{message: msg, numRetries: 3}
	inCh1 <- env

	// Verify that the messages sent to node1 exits node2
	select {
	case received := <-outCh2:
		assert.Equal(t, msg, received.message, "Expected to receive the same message in node2 input channel")
	case <-time.After(1 * time.Millisecond):
		t.Error("Expected message in node2 output channel")
	}

	// Send a message that will cause an error
	msgError := &testMessage{Content: "error"}
	envError := &Envelope[testMessage]{message: msgError, numRetries: 3}
	inCh1 <- envError

	// Verify that the error is handled and the message is sent to the event channel
	select {
	case ev := <-graph.Supervisors[SupervisorName].events:
		assert.Equal(t, ErrorLevelError, ev.Level)
		assert.Contains(t, ev.Event.Error(), "test error")
		assert.Equal(t, msgError, ev.Message)
	case <-time.After(1 * time.Millisecond):
		t.Error("Timeout waiting for error event")
	}

	// Clean up the graph
	graph.Stop()
}
