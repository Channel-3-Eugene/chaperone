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

type testHandler struct {
	outChannelName string
}

func (h *testHandler) Handle(msg *testMessage) (string, error) {
	if msg.Content == "error" {
		return "", NewEvent[testMessage](ErrorLevelError, errors.New("test error"), nil)
	}
	return h.outChannelName, nil
}

type testSupervisorHandler struct{}

func (h *testSupervisorHandler) Handle(msg *testMessage) (string, error) {
	return "supervised", nil
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

func TestChaperone_EndToEnd(t *testing.T) {
	// Create the graph
	ctx := context.Background()

	SupervisorName := "supervisor1"
	Node1Name := "node1"
	Node1WorkerName := "worker1"
	Node2Name := "node2"
	Node2WorkerName := "worker2"
	inputChannelName := "input"
	outputChannelName := "outChannel"

	graph := NewGraph[testMessage](ctx).
		AddSupervisor(SupervisorName, &testSupervisorHandler{}).
		AddNode(SupervisorName, Node1Name, &testHandler{outChannelName: Node1Name + ":" + outputChannelName}).
		AddWorkers(Node1Name, 1, Node1WorkerName).
		AddNode(SupervisorName, Node2Name, &testHandler{outChannelName: Node2Name + ":" + outputChannelName}).
		AddWorkers(Node2Name, 1, Node2WorkerName).
		AddEdge("", "", Node1Name, inputChannelName, 10).
		AddEdge(Node1Name, outputChannelName, Node2Name, inputChannelName, 10).
		AddEdge(Node2Name, outputChannelName, "", "final", 10).
		Start()

	// Set up input and output channels for the nodes
	inCh1 := graph.Nodes[Node1Name].inputChans["input"]
	outCh2 := graph.Nodes[Node2Name].outputChans[Node2Name+":"+outputChannelName].goChans[":final"]

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
