package chaperone

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestKillContext_WithKill(t *testing.T) {
	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx, kill := WithKill(parentCtx)

	// Test if Die channel is present
	select {
	case <-Die(ctx):
		t.Error("Die channel should not be closed initially")
	default:
		// Expected case
	}

	// Test if Die channel is closed after calling kill()
	kill()
	select {
	case <-Die(ctx):
		// Expected case
	default:
		t.Error("Die channel should be closed after calling kill")
	}
}

func TestKillContext_Die(t *testing.T) {
	parentCtx := context.Background()
	ctx, kill := WithKill(parentCtx)

	// Die should return a closed channel if not present
	assert.NotNil(t, Die(ctx))
	select {
	case <-Die(ctx):
		t.Error("Die channel should not be closed initially")
	default:
		// Expected case
	}

	// Close the Die channel and test
	kill()
	select {
	case <-Die(ctx):
		// Expected case
	default:
		t.Error("Die channel should be closed after calling kill")
	}
}

func TestKillContext_IntegrationWithKill(t *testing.T) {
	parentCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx, kill := WithKill(parentCtx)

	done := make(chan struct{})

	go func() {
		select {
		case <-ctx.Done():
			t.Log("Received Done signal")
		case <-Die(ctx):
			t.Log("Received Die signal")
		}
		close(done)
	}()

	// Test receiving Done signal
	cancel()
	select {
	case <-done:
		// Expected case
	case <-time.After(1 * time.Second):
		t.Error("Expected to receive Done signal")
	}

	// Reset the done channel
	done = make(chan struct{})
	ctx, kill = WithKill(context.Background())
	go func() {
		select {
		case <-ctx.Done():
			t.Log("Received Done signal")
		case <-Die(ctx):
			t.Log("Received Die signal")
		}
		close(done)
	}()

	// Test receiving Die signal
	kill()
	select {
	case <-done:
		// Expected case
	case <-time.After(1 * time.Second):
		t.Error("Expected to receive Die signal")
	}
}
