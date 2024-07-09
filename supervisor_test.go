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

func (h supervisorNodeHandler) Handle(ctx context.Context, env *Envelope[supervisorNodeTestMessage]) error {
	if env.Message.Content == "error" {
		env.OutChan = nil
		return errors.New("test error")
	}

	return nil
}

type supervisorHandler struct{}

func (h supervisorHandler) Handle(ctx context.Context, env *Envelope[supervisorNodeTestMessage]) error {
	if env.Event.Level == ErrorLevelCritical {
		panic(env.Event)
	}

	if env.Event.Level == ErrorLevelError {
		env.OutChan = nil
	}
	return nil
}

func TestSupervisor_NewSupervisor(t *testing.T) {
	t.Run("creates a new supervisor with the given name", func(t *testing.T) {
		handler := supervisorHandler{}
		parentSupervisor := NewSupervisor[supervisorNodeTestMessage](context.Background(), "parent supervisor", handler)
		childSupervisor := NewSupervisor[supervisorNodeTestMessage](context.Background(), "child supervisor", handler)
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
		supervisor := NewSupervisor[supervisorNodeTestMessage](context.Background(), "TestSupervisor", supervisorHandler)
		assert.NotNil(t, supervisor.Nodes)

		ctx := context.Background()
		nodeHandler := supervisorNodeHandler{}
		node := NewNode[supervisorNodeTestMessage](ctx, "TestNode", nodeHandler)
		supervisor.AddNode(node)

		assert.Contains(t, supervisor.Nodes, "TestNode")
		assert.Equal(t, supervisor.Events, node.EventChan)
	})
}
