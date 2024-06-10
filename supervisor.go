package chaperone

import (
	"context"
)

type SupervisorContextKey struct{}

type Supervisor[E *Envelope[I], I any] struct {
	ID               string
	Node             *Node[E, I]
	Children         []Supervisable[E, I]
	ErrorBucket      Channel[E, I]
	FailureThreshold int
}

type Supervisable[E *Envelope[I], I any] interface {
	Start(ctx context.Context)
	Stop(ctx context.Context)
}

func NewSupervisor[E *Envelope[I], I any](id string, node *Node[E, I], children []Supervisable[E, I], errorBucket Channel[E, I], failureThreshold int) *Supervisor[E, I] {
	return &Supervisor[E, I]{
		ID:               id,
		Node:             node,
		Children:         children,
		ErrorBucket:      errorBucket,
		FailureThreshold: failureThreshold,
	}
}

func (s *Supervisor[E, I]) Start(ctx context.Context) {
	ctx = context.WithValue(ctx, SupervisorContextKey{}, s)
	s.Node.Start(ctx)
	for _, child := range s.Children {
		child.Start(ctx)
	}
}

func (s *Supervisor[E, I]) Stop(ctx context.Context) {
	s.Node.Stop(ctx)
	for _, child := range s.Children {
		child.Stop(ctx)
	}
}

func (s *Supervisor[E, I]) HandleError(ctx context.Context, ev Error[E, I], msg *Envelope[I]) {
	s.ErrorBucket <- msg
	s.Node.errorCount[msg.inputChan]++
	if s.Node.errorCount[msg.inputChan] >= s.FailureThreshold {
		s.Node.shortCircuit[msg.inputChan] = true
		s.Node.errorCount[msg.inputChan] = 0
		s.Node.Send(msg, s.Node.OutputChans[DEVNULL])
		return
	}

	msg.DecrementRetry()
	if msg.RetryCount() > 0 {
		s.Node.Retry(ctx, msg, s.Node.InputChans[msg.inputChan])
	} else {
		s.Node.Send(msg, s.Node.OutputChans[DEVNULL])
	}
}
