package chaperone

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type outmuxTestMessage struct {
	Data string
}

func (m outmuxTestMessage) String() string {
	return m.Data
}

func TestOutmux_NewOutMux(t *testing.T) {
	outMux := NewOutMux("test")
	assert.NotNil(t, outMux.OutChans, "OutChans should not be nil")
	assert.NotNil(t, outMux.GoChans, "GoChans should not be nil")
	if outMux.GoChans == nil {
		t.Fatal("GoChans map is nil")
	}
}

func TestOutmux_AddChannel(t *testing.T) {
	outMux := NewOutMux("test")
	outEdge := NewEdge("test", nil, nil, 10, 1)
	outMux.AddChannel(outEdge)

	assert.Equal(t, outEdge, outMux.GoChans["test"], "Expected new edge to be added to OutChans")
	assert.Contains(t, outMux.GoChans, "test", "Expected new edge to be added to GoChans")
}

func TestOutmux_SendOnce(t *testing.T) {
	outMux := NewOutMux("test")
	outMux.AddChannel(NewEdge("test1", nil, nil, 10, 1))
	msg := &Envelope[outmuxTestMessage]{Message: outmuxTestMessage{Data: "TestOutmux_Send"}}
	outMux.Send(msg)

	select {
	case receivedMsg := <-outMux.GoChans["test1"].GetChannel():
		if receivedMsg.String() != "TestOutmux_Send" {
			t.Fatalf("Expected 'test message', got '%s'", receivedMsg.String())
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message on ch1")
	}
}

func TestOutmux_AddChannelTwice(t *testing.T) {
	outMux := NewOutMux("test")
	outEdge := NewEdge("test", nil, nil, 10, 1)
	outMux.AddChannel(outEdge)
	outMux.AddChannel(outEdge)

	if len(outMux.GoChans) != 1 {
		t.Fatalf("Expected 1 channel, got %d", len(outMux.GoChans))
	}
}

func TestOutmux_SendToNonExistentChannel(t *testing.T) {
	outMux := NewOutMux("test")
	msg := &Envelope[outmuxTestMessage]{Message: outmuxTestMessage{Data: "test message"}}
	outMux.Send(msg)

	// If no panic occurs, test passes
}
