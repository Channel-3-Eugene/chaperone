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
	outMux := NewOutMux[outmuxTestMessage]("test")
	assert.NotNil(t, outMux.OutChans, "OutChans should not be nil")
	assert.NotNil(t, outMux.GoChans, "GoChans should not be nil")
	if outMux.GoChans == nil {
		t.Fatal("GoChans map is nil")
	}
}

func TestOutmux_AddChannel(t *testing.T) {
	outMux := NewOutMux[outmuxTestMessage]("test")
	ch := make(chan *Envelope[outmuxTestMessage], 5)
	outMux.AddChannel("test", ch)

	if _, exists := outMux.GoChans["test"]; !exists {
		t.Fatal("Channel 'test' not added to GoChans")
	}
}

func TestOutmux_Send(t *testing.T) {
	outMux := NewOutMux[outmuxTestMessage]("test")
	ch1 := make(chan *Envelope[outmuxTestMessage], 5)
	ch2 := make(chan *Envelope[outmuxTestMessage], 5)
	outMux.AddChannel("test1", ch1)
	outMux.AddChannel("test2", ch2)

	msg := &Envelope[outmuxTestMessage]{Message: outmuxTestMessage{Data: "test message"}}
	outMux.Send(msg)

	select {
	case receivedMsg := <-ch1:
		if receivedMsg.Message.Data != "test message" {
			t.Fatalf("Expected 'test message', got '%s'", receivedMsg.Message.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message on ch1")
	}

	select {
	case receivedMsg := <-ch2:
		if receivedMsg.Message.Data != "test message" {
			t.Fatalf("Expected 'test message', got '%s'", receivedMsg.Message.Data)
		}
	case <-time.After(time.Second):
		t.Fatal("Timeout waiting for message on ch2")
	}
}

func TestOutmux_AddChannelTwice(t *testing.T) {
	outMux := NewOutMux[outmuxTestMessage]("test")
	ch := make(chan *Envelope[outmuxTestMessage], 5)
	outMux.AddChannel("test", ch)
	outMux.AddChannel("test", ch)

	if len(outMux.GoChans) != 1 {
		t.Fatalf("Expected 1 channel, got %d", len(outMux.GoChans))
	}
}

func TestOutmux_SendToNonExistentChannel(t *testing.T) {
	outMux := NewOutMux[outmuxTestMessage]("test")
	msg := &Envelope[outmuxTestMessage]{Message: outmuxTestMessage{Data: "test message"}}
	outMux.Send(msg)

	// If no panic occurs, test passes
}
