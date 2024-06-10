package chaperone

import (
	"context"
	"testing"
)

// TestItem is a simple implementation of the ItemInterface for testing purposes.
type TestItem struct {
	data string
}

// Implement the ItemInterface for TestItem with a pointer receiver.
func (i TestItem) String() string {
	return i.data
}

// TestEnvelope tests the basic functionality of the Envelope struct.
func TestMessaging_Envelope(t *testing.T) {
	item := &TestItem{data: "test data"}
	retryCount := 3

	envelope := NewEnvelope(item, retryCount)

	if envelope.Item() != item {
		t.Errorf("expected item to be %#v, got %#v", item, envelope.Item())
	}

	if envelope.RetryCount() != retryCount {
		t.Errorf("expected retry count to be %d, got %d", retryCount, envelope.RetryCount())
	}

	envelope.DecrementRetry()
	if envelope.RetryCount() != retryCount-1 {
		t.Errorf("expected retry count to be %d, got %d", retryCount-1, envelope.RetryCount())
	}

	newItem := &TestItem{data: "new data"}
	envelope.SetItem(newItem)
	if envelope.Item() != newItem {
		t.Errorf("expected item to be %#v, got %#v", newItem, envelope.Item())
	}
}

// TestChannel tests the basic functionality of the Channel type alias.
func TestMessaging_Channel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	devNullChan := NewDevNullChannel[*Envelope[TestItem], TestItem](ctx)

	item := TestItem{data: "test data"}
	envelope := NewEnvelope(item, 3)

	go func() {
		devNullChan.Send(envelope)
	}()

	receivedEnvelope := devNullChan.Receive()
	if receivedEnvelope != envelope {
		t.Errorf("expected to receive %#v, got %#v", envelope, receivedEnvelope)
	}
}
