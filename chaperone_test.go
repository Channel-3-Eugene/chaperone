package chaperone

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testMessage struct {
	Content string
}

func (m testMessage) String() string {
	return m.Content
}

type testHandler struct {
	outChannelName string
}

func (h *testHandler) Handle(ctx context.Context, env *Envelope[testMessage]) error {
	if env.Message.Content == "error" {
		return NewEvent[testMessage](ErrorLevelError, errors.New("test error"), nil)
	}

	env.OutChan = env.CurrentNode.OutputChans[h.outChannelName]
	return nil
}

type testSupervisorHandler struct{}

func (h *testSupervisorHandler) Handle(ctx context.Context, _ *Envelope[testMessage]) error {
	return nil
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
	Node2Name := "node2"
	inputChannelName := "input"
	outputChannelName := "output"

	fmt.Print("setting up graph\n")

	graph := NewGraph[testMessage](ctx, "graph", &Config{}).
		AddSupervisor(SupervisorName, &testSupervisorHandler{}).
		AddNode(SupervisorName, Node1Name, &testHandler{outChannelName: Node1Name + ":" + outputChannelName}).
		AddNode(SupervisorName, Node2Name, &testHandler{outChannelName: Node2Name + ":" + outputChannelName}).
		AddEdge("", "start", Node1Name, inputChannelName, 10, 1).
		AddEdge(Node1Name, outputChannelName, Node2Name, inputChannelName, 10, 1).
		AddEdge(Node2Name, outputChannelName, "", "final", 10, 1).
		Start()

	for _, node := range graph.Nodes {
		fmt.Printf("Node: %+v\n", node.InputChans)
	}

	for _, edge := range graph.Edges {
		fmt.Printf("Edge: %+v\n", edge)
	}

	// Set up input and output channels for the nodes
	inCh1 := graph.Nodes[Node1Name].InputChans[Node1Name+":"+inputChannelName]
	fmt.Printf("Node1 input channel: %+v\n", inCh1)
	outCh2 := graph.Nodes[Node2Name].OutputChans[Node2Name+":"+outputChannelName].GoChans[":final"]
	fmt.Printf("Node2 output channel: %+v\n", outCh2)

	// Send random messages to the input channel of node 1
	msgContent, err := randomString(10)
	assert.NoError(t, err)
	msg := testMessage{Content: msgContent}
	env := &Envelope[testMessage]{Message: msg, NumRetries: 3}
	fmt.Printf("Sending message to Node1 input channel: %+v\n", env)
	inCh1 <- env

	// Verify that the messages sent to node1 exits node2
	select {
	case received := <-outCh2:
		fmt.Printf("Received message in node2 output channel: %+v\n", received)
		assert.Equal(t, msg, received.Message, "Expected to receive the same message in node2 input channel")
	case <-time.After(1 * time.Second):
		t.Error("Expected message in node2 output channel")
	}

	// Send a message that will cause an error
	msgError := testMessage{Content: "error"}
	envError := &Envelope[testMessage]{Message: msgError, NumRetries: 3}
	fmt.Printf("Sending error message to Node1 input channel: %+v\n", envError)
	inCh1 <- envError

	// Verify that the error is handled and the message is sent to the event channel
	select {
	case ev := <-graph.Supervisors[SupervisorName].Events:
		assert.Equal(t, ErrorLevelError, ev.Level)
		assert.Contains(t, ev.Event.Error(), "test error")
		assert.Equal(t, msgError, ev.Envelope.Message)
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for error event")
	}

	// Clean up the graph
	graph.Stop()
}
