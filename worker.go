package chaperone

import "context"

type WorkerInterface[E *Envelope[I], I any] interface {
	Start(ctx context.Context)
	Stop()
	Handle(ctx context.Context, msg E) (E, Channel[E, I], error)
	Name() string
	Status() string
}

type WorkerFactoryInterface[E *Envelope[I], I any] interface {
	CreateWorker(id int) WorkerInterface[E, I]
}
