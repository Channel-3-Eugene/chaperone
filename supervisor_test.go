package chaperone

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type supervisorNodeTestMessage struct {
	Content string
}

func (s supervisorNodeTestMessage) String() string {
	return s.Content
}

type supervisorNodeHandler struct{}

func (h supervisorNodeHandler) Start(ctx context.Context) error {
	return nil
}

func (h supervisorNodeHandler) Handle(ctx context.Context, env *Envelope[supervisorNodeTestMessage]) (*Envelope[supervisorNodeTestMessage], error) {
	if env.Message.Content == "error" {
		evt := NewEvent[supervisorNodeTestMessage, supervisorNodeTestMessage](ErrorLevelCritical, errors.New("test error"), env)
		return nil, evt
	}

	return env, nil
}

type supervisorHandler struct{}

func (h supervisorHandler) Start(ctx context.Context) error {
	return nil
}

func (h supervisorHandler) Handle(ctx context.Context, evt *Event[supervisorNodeTestMessage, supervisorNodeTestMessage]) error {
	if evt != nil && evt.Level == ErrorLevelCritical {
		newEvt := NewEvent[supervisorNodeTestMessage, supervisorNodeTestMessage](ErrorLevelError, errors.New("supervised"), evt.Envelope)
		panic(newEvt)
	}

	return nil
}

func TestSupervisor_NewSupervisor(t *testing.T) {
	t.Run("creates a new supervisor with the given name", func(t *testing.T) {
		handler := supervisorHandler{}
		parentSupervisor := NewSupervisor[supervisorNodeTestMessage, supervisorNodeTestMessage](context.Background(), "parent supervisor", handler)
		childSupervisor := NewSupervisor[supervisorNodeTestMessage, supervisorNodeTestMessage](context.Background(), "child supervisor", handler)
		parentSupervisor.AddSupervisor(childSupervisor)

		assert.NotNil(t, parentSupervisor)
		assert.Equal(t, "parent supervisor", parentSupervisor.Name)
		assert.NotNil(t, parentSupervisor.Nodes)
		assert.Len(t, parentSupervisor.Supervisors, 1)
		assert.NotNil(t, parentSupervisor.Events)

		assert.NotNil(t, childSupervisor)
		assert.Equal(t, "child supervisor", childSupervisor.Name)
		assert.NotNil(t, childSupervisor.Nodes)
		assert.NotNil(t, childSupervisor.Parent)
		assert.NotNil(t, childSupervisor.Events)
	})
}

func TestSupervisor_AddNode(t *testing.T) {
	t.Run("adds a node to the supervisor", func(t *testing.T) {
		supervisorHandler := supervisorHandler{}
		supervisor := NewSupervisor[supervisorNodeTestMessage, supervisorNodeTestMessage](context.Background(), "TestSupervisor", supervisorHandler)
		assert.NotNil(t, supervisor.Nodes)

		ctx := context.Background()
		nodeHandler := supervisorNodeHandler{}
		node := NewNode[supervisorNodeTestMessage, supervisorNodeTestMessage](ctx, "TestNode", nodeHandler)
		supervisor.AddNode(node)

		assert.Contains(t, supervisor.Nodes, "TestNode")
		assert.Equal(t, supervisor.Events, node.Events)
	})
}

func TestSupervisor_RestartNode(t *testing.T) {
	t.Run("restarts a node", func(t *testing.T) {
		supervisorHandler := supervisorHandler{}
		supervisor := NewSupervisor[supervisorNodeTestMessage, supervisorNodeTestMessage](context.Background(), "TestSupervisor", supervisorHandler)

		ctx := context.Background()
		nodeHandler := supervisorNodeHandler{}
		node := NewNode[supervisorNodeTestMessage, supervisorNodeTestMessage](ctx, "TestNode", nodeHandler)
		supervisor.AddNode(node)

		inEdge := NewEdge[supervisorNodeTestMessage, supervisorNodeTestMessage]("input", nil, node, 10, 1)
		node.AddInput("input", inEdge)
		node.AddWorkers(inEdge, 3, "worker")

		assert.Len(t, node.WorkerPool["input"], 3)

		supervisor.Start()
		time.Sleep(20 * time.Microsecond)

		env := NewEnvelope[supervisorNodeTestMessage](supervisorNodeTestMessage{Content: "error"}, 2)
		node.In["input"].Channel <- env
		assert.Len(t, node.In["input"].Channel, 1)

		time.Sleep(1 * time.Millisecond)
		assert.Len(t, node.In["input"], 0)
		assert.Len(t, node.WorkerPool["input"], 3)

		// TODO: Figure out how to make sure both the node handler and the supervisor handler touched env without race conditions or mutexes
	})
}
