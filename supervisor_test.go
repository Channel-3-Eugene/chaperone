package chaperone

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type supervisorTestMessage struct {
	Content string
}

type supervisorTestHandler struct{}

func (h *supervisorTestHandler) Handle(msg *supervisorTestMessage) (string, error) {
	if msg.Content == "error" {
		return "", errors.New("test error")
	}
	return "outChannel", nil
}

func TestNewSupervisor(t *testing.T) {
	t.Run("creates a new supervisor with the given name", func(t *testing.T) {
		name := "TestSupervisor"
		supervisor := NewSupervisor[supervisorTestMessage](name)

		assert.NotNil(t, supervisor)
		assert.Equal(t, name, supervisor.Name)
		assert.NotNil(t, supervisor.Nodes)
		assert.NotNil(t, supervisor.events)
	})
}

func TestSupervisor_AddNode(t *testing.T) {
	t.Run("adds a node to the supervisor", func(t *testing.T) {
		supervisor := NewSupervisor[supervisorTestMessage]("TestSupervisor")
		assert.NotNil(t, supervisor.Nodes)

		ctx, cancel := context.WithCancelCause(context.Background())
		handler := &supervisorTestHandler{}
		node := newNode(ctx, cancel, "TestNode", handler, 3)

		supervisor.AddNode(node)

		assert.Contains(t, supervisor.Nodes, "TestNode")
		assert.Equal(t, supervisor.events, node.eventChan)
	})
}
