package chaperone

import (
	"context"
)

// killContext embeds context.Context and adds a die channel
type killContext struct {
	context.Context
	die chan struct{}
}

// WithKill returns a copy of the parent context with an added Die channel
func WithKill(parent context.Context) (context.Context, func()) {
	ctx := &killContext{
		Context: parent,
		die:     make(chan struct{}),
	}
	return ctx, func() {
		close(ctx.die)
	}
}

// Die retrieves the Die channel from the context
func Die(ctx context.Context) <-chan struct{} {
	if kc, ok := ctx.(*killContext); ok {
		return kc.die
	}
	// Return a closed channel if Die is not present in context
	closedChan := make(chan struct{})
	close(closedChan)
	return closedChan
}
