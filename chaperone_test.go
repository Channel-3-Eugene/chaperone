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

func (h *testHandler) Start(context.Context) error {
	return nil
}

func (h *testHandler) Handle(ctx context.Context, env Message) (Message, error) {
	if env.String() == "error" {
		return nil, NewEvent(ErrorLevelError, errors.New("test error"), env)
	}

	return env, nil
}

type testSupervisorHandler struct{}

func (h *testSupervisorHandler) Start(context.Context) error {
	return nil
}

func (h *testSupervisorHandler) Handle(ctx context.Context, evt Message) error {
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

	ParentSupervisorName := "parent supervisor"
	ParentSupervisor := NewSupervisor(ctx, ParentSupervisorName, &testSupervisorHandler{})
	ChildSupervisorName := "child supervisor"
	ChildSupervisor := NewSupervisor(ctx, ChildSupervisorName, &testSupervisorHandler{})
	Node1Name := "node1"
	Node1 := NewNode[testMessage, testMessage](ctx, Node1Name, &testHandler{outChannelName: "middle"})
	fmt.Printf("Created node1: %#v\n\n", Node1)
	startEdge := NewEdge("start", nil, Node1, 10, 1)
	Node2Name := "node2"
	Node2 := NewNode[testMessage, testMessage](ctx, Node2Name, &testHandler{outChannelName: "end"})
	middleEdge := NewEdge("middle", Node1, Node2, 10, 1)
	endEdge := NewEdge("end", Node2, nil, 10, 1)

	fmt.Print("setting up graph\n")

	graph := NewGraph(ctx, "graph", &Config{}).
		AddSupervisor(nil, ParentSupervisor).
		AddSupervisor(ParentSupervisor, ChildSupervisor).
		AddEdge(startEdge).
		AddNode(Node1).
		AddEdge(middleEdge).
		AddNode(Node2).
		AddEdge(endEdge).
		Start()

	// Send random messages to the input channel of node 1
	msgContent, err := randomString(10)
	assert.NoError(t, err)
	msg := testMessage{Content: msgContent}
	env := &Envelope[testMessage]{Message: msg, NumRetries: 3}
	fmt.Printf("Sending message to Node1 input channel: %+v\n", env)
	startEdge.GetChannel() <- env

	// Verify that the messages sent to node1 exits node2
	select {
	case received := <-endEdge.GetChannel():
		fmt.Printf("Received message in node2 output channel: %+v\n", received)
		assert.Equal(t, msg.String(), received.String(), "Expected to receive the same message in node2 input channel")
	case <-time.After(1 * time.Second):
		t.Error("Expected message in node2 output channel")
	}

	// Send a message that will cause an error
	msgError := testMessage{Content: "error"}
	envError := &Envelope[testMessage]{Message: msgError, NumRetries: 3}
	fmt.Printf("Sending error message to Node1 input channel: %+v\n", envError)
	startEdge.GetChannel() <- envError

	// Verify that the error is handled and the message is sent to the event channel
	select {
	case evt := <-ChildSupervisor.Events.GetChannel():
		ev, ok := evt.(*Event)
		assert.True(t, ok)
		assert.Equal(t, ErrorLevelError, ev.Level())
		assert.Contains(t, ev.Error(), "test error")
		assert.Equal(t, msgError.String(), ev.String())
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for error event")
	}

	// Clean up the graph
	graph.Stop()
}
