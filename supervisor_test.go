package chaperone

import (
	"context"
	"errors"
	"testing"

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

func (h supervisorNodeHandler) Handle(ctx context.Context, env Message) (Message, error) {
	if env.String() == "error" {
		evt := NewEvent(ErrorLevelCritical, errors.New("test error"), env)
		return nil, evt
	}

	return env, nil
}

func (h supervisorNodeHandler) Stop() {}

type supervisorHandler struct{}

func (h supervisorHandler) Start(ctx context.Context) error {
	return nil
}

func (h supervisorHandler) Handle(ctx context.Context, evt Message) error {
	ev, ok := evt.(*Event)
	if !ok {
		return errors.New("invalid event type")
	}
	if ev != nil && ev.Level() == ErrorLevelCritical {
		newEvt := NewEvent(ErrorLevelError, errors.New("supervised"), ev.envelope)
		panic(newEvt)
	}

	return nil
}

func TestSupervisor_NewSupervisor(t *testing.T) {
	t.Run("creates a new supervisor with the given name", func(t *testing.T) {
		handler := supervisorHandler{}
		parentSupervisor := NewSupervisor("parent supervisor", handler)
		childSupervisor := NewSupervisor("child supervisor", handler)
		parentSupervisor.AddChildSupervisor(childSupervisor)

		assert.NotNil(t, parentSupervisor)
		assert.Equal(t, "parent supervisor", parentSupervisor.Name())
		assert.NotNil(t, parentSupervisor.Nodes)
		assert.Len(t, parentSupervisor.Supervisors, 1)
		assert.NotNil(t, parentSupervisor.Events)

		assert.NotNil(t, childSupervisor)
		assert.Equal(t, "child supervisor", childSupervisor.Name())
		assert.NotNil(t, childSupervisor.Nodes)
		assert.NotNil(t, childSupervisor.ParentEvents)
		assert.NotNil(t, childSupervisor.Events)
	})
}

func TestSupervisor_AddNode(t *testing.T) {
	t.Run("adds a node to the supervisor", func(t *testing.T) {
		supervisorHandler := supervisorHandler{}
		supervisor := NewSupervisor("TestSupervisor", supervisorHandler)
		assert.NotNil(t, supervisor.Nodes)

		nodeHandler := supervisorNodeHandler{}
		node := NewNode[supervisorNodeTestMessage, supervisorNodeTestMessage]("TestNode", nodeHandler, nil)
		supervisor.addNode(node)

		assert.Contains(t, supervisor.Nodes, "TestNode")
		assert.Equal(t, supervisor.Events, node.Events)
	})
}

func TestSupervisor_RestartNode(t *testing.T) {
	// TODO: Figure out how to make sure both the node handler and the supervisor handler touched env without race conditions or mutexes
}
