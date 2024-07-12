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

func (h *testHandler) Handle(ctx context.Context, env *Envelope[testMessage]) (*Envelope[testMessage], error) {
	if env.Message.Content == "error" {
		return nil, NewEvent[testMessage, testMessage](ErrorLevelError, errors.New("test error"), env)
	}

	return env, nil
}

type testSupervisorHandler struct{}

func (h *testSupervisorHandler) Start(context.Context) error {
	return nil
}

func (h *testSupervisorHandler) Handle(ctx context.Context, evt *Event[testMessage, testMessage]) error {
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
	ParentSupervisor := NewSupervisor[testMessage, testMessage](ctx, ParentSupervisorName, &testSupervisorHandler{})
	ChildSupervisorName := "child supervisor"
	ChildSupervisor := NewSupervisor[testMessage, testMessage](ctx, ChildSupervisorName, &testSupervisorHandler{})
	Node1Name := "node1"
	Node1 := NewNode[testMessage, testMessage](ctx, Node1Name, &testHandler{outChannelName: "middle"})
	startEdge := NewEdge[testMessage, testMessage, testMessage]("start", nil, Node1, 10, 1)
	Node2Name := "node2"
	Node2 := NewNode[testMessage, testMessage](ctx, Node2Name, &testHandler{outChannelName: "end"})
	middleEdge := NewEdge[testMessage, testMessage, testMessage]("middle", Node1, Node2, 10, 1)
	endEdge := NewEdge[testMessage, testMessage, testMessage]("end", Node2, nil, 10, 1)

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
	startEdge.Channel <- env

	// Verify that the messages sent to node1 exits node2
	select {
	case received := <-endEdge.Channel:
		fmt.Printf("Received message in node2 output channel: %+v\n", received)
		assert.Equal(t, msg, received.Message, "Expected to receive the same message in node2 input channel")
	case <-time.After(1 * time.Second):
		t.Error("Expected message in node2 output channel")
	}

	// Send a message that will cause an error
	msgError := testMessage{Content: "error"}
	envError := &Envelope[testMessage]{Message: msgError, NumRetries: 3}
	fmt.Printf("Sending error message to Node1 input channel: %+v\n", envError)
	startEdge.Channel <- envError

	// Verify that the error is handled and the message is sent to the event channel
	select {
	case ev := <-graph.Supervisors[ChildSupervisorName].(*Supervisor[testMessage, testMessage]).Events:
		assert.Equal(t, ErrorLevelError, ev.Level)
		assert.Contains(t, ev.Event.Error(), "test error")
		assert.Equal(t, msgError, ev.Envelope.Message)
	case <-time.After(1 * time.Second):
		t.Error("Timeout waiting for error event")
	}

	// Clean up the graph
	graph.Stop()
}
