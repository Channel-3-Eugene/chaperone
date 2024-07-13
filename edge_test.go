package chaperone

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type edgeTestMessage struct {
	Content string
}

func (m edgeTestMessage) String() string {
	return m.Content
}

type edgeTestHandler struct {
	outChannelName string
}

func (h *edgeTestHandler) Start(context.Context) error {
	return nil
}

func (h *edgeTestHandler) Handle(ctx context.Context, env Message) (Message, error) {
	if env.String() == "error" {
		return nil, errors.New("test error")
	}

	return env, nil
}

func TestEdge_NewEdge(t *testing.T) {
	ctx := context.Background()

	node1 := NewNode[edgeTestMessage, edgeTestMessage](ctx, "node1", &edgeTestHandler{outChannelName: "output"})
	node2 := NewNode[edgeTestMessage, edgeTestMessage](ctx, "node2", &edgeTestHandler{outChannelName: "output"})

	edgeName := "edge1"
	edge := NewEdge(edgeName, node1, node2, 10, 1)
	assert.NotNil(t, edge)
	assert.NotNil(t, edge.GetChannel())
	assert.Equal(t, 10, cap(edge.GetChannel()))

	assert.Equal(t, "node1:output", node1.Out.Name)
	assert.Contains(t, node2.In, edgeName)

	outChannel := node1.Out.GoChans["node2:input"]
	inChannel := node2.In["node2:input"]
	assert.Equal(t, outChannel, inChannel)
}
