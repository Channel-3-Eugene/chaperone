package chaperone

import (
	"context"
	"fmt"
	"runtime"
	"sync"
)

type Node[E *Envelope[I], I any] struct {
	ID         string
	Name       string
	Factory    WorkerFactoryInterface[E, I]
	RetryLimit int

	InputChans  map[int]Channel[E, I]
	OutputChans map[int]Channel[E, I]

	workerPool   map[int]WorkerInterface[E, I]
	errorCount   map[int]int
	shortCircuit map[int]bool

	eventChan chan *Error[E, I]

	wg     *sync.WaitGroup
	cancel context.CancelFunc
}

func NewNode[E *Envelope[I], I any](id, name string, workerCount, retryLimit int, factory WorkerFactoryInterface[E, I]) Node[E, I] {
	node := Node[E, I]{
		ID:         id,
		Name:       name,
		Factory:    factory,
		RetryLimit: retryLimit,

		workerPool:   make(map[int]WorkerInterface[E, I]),
		errorCount:   make(map[int]int),
		shortCircuit: make(map[int]bool),

		InputChans:  make(map[int]Channel[E, I]),
		OutputChans: make(map[int]Channel[E, I]),

		eventChan: nil, // Supervisor can attach to this

		wg: &sync.WaitGroup{},
	}

	for i := range workerCount {
		worker := factory.CreateWorker(i)
		node.workerPool[i] = worker
	}

	return node
}

func (n *Node[E, I]) AddInputChannel(id int) {
	inCh := make(Channel[E, I])
	n.InputChans[id] = inCh
}

func (n *Node[E, I]) RemoveInputChannel(id int) {
	delete(n.InputChans, id)
}

func (n *Node[E, I]) AddOutputChannel(id int) {
	outCh := make(Channel[E, I])
	n.OutputChans[id] = outCh
}

func (n *Node[E, I]) RemoveOutputChannel(id int) {
	delete(n.OutputChans, id)
}

func (n *Node[E, I]) SetEventChan(evCh chan *Error[E, I]) {
	n.eventChan = evCh
}

func (n *Node[E, I]) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	n.cancel = cancel

	for _, w := range n.workerPool {
		w.Start(ctx)
		n.wg.Add(1)
		go n.startWorker(ctx, n.wg, w)
	}

}

func (n *Node[E, I]) Stop(ctx context.Context) {
	for _, w := range n.workerPool {
		w.Stop()
	}
	n.cancel()
}

func (n *Node[E, I]) startWorker(ctx context.Context, wg *sync.WaitGroup, w WorkerInterface[E, I]) {
	defer wg.Done()

	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case *Envelope[I]:
				err := NewError(ErrorLevelCritical, fmt.Errorf("worker %s panicked", w.Name()), w, x)
				n.handleWorkerError(ctx, err)
			case runtime.Error:
				err := NewError(ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", w.Name(), x), w, nil)
				n.handleWorkerError(ctx, err)
			default:
				err := NewError(ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", w.Name(), x), w, nil)
				n.handleWorkerError(ctx, err)
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return
		default:
			for _, inputChan := range n.InputChans {
				select {
				case msg, ok := <-inputChan:
					if !ok {
						return // channels getting shut down means a hard stop
					}

					// msg.DecrementRetry()

					newMsg, outCh, err := w.Handle(ctx, msg)

					if err != nil {
						newErr := NewError(ErrorLevelError, err, w, msg)
						n.handleWorkerError(ctx, newErr)
					} else if outCh != nil {
						n.Send(newMsg, outCh)
					} else {
						if devnull, exists := n.OutputChans[DEVNULL]; exists {
							devnull <- newMsg
						}
					}
				}
			}
		}
	}
}

func (n *Node[E, I]) handleWorkerError(ctx context.Context, err *Error[E, I]) {
	// msg := err.Message
	// n.errorCount[msg.inputChan]++
	// if n.errorCount[msg.inputChan] >= n.RetryLimit {
	// 	n.shortCircuit[msg.inputChan] = true
	// 	n.errorCount[msg.inputChan] = 0
	// 	n.SendToSupervisor(ctx, err)
	// 	return
	// }

	// msg.DecrementRetry()
	// if msg.retryCount > 0 {
	// 	n.Retry(ctx, msg, msg.inputChan)
	// } else if devnull, exists := n.OutputChans[DEVNULL]; exists {
	// 	n.Send(msg, devnull)
	// }
}

func (n *Node[E, I]) Send(msg *Envelope[I], toChan Channel[E, I]) {
	if toChan != nil {
		toChan.Send(msg)
	} else {
		if devnull, exists := n.OutputChans[DEVNULL]; exists {
			devnull <- msg
		}
	}
}

func (n *Node[E, I]) Retry(ctx context.Context, msg *Envelope[I], inChan Channel[E, I]) {
	if msg.RetryCount() < 1 {
		if devnull, exists := n.OutputChans[DEVNULL]; exists {
			devnull <- msg
		}
		return
	}
	msg.DecrementRetry()
	if inChan != nil {
		n.Send(msg, inChan)
	} else {
		if devnull, exists := n.OutputChans[DEVNULL]; exists {
			devnull <- msg
		}
	}
}

func (n *Node[E, I]) SendToSupervisor(ctx context.Context, ev *Error[E, I]) {
	if n.eventChan != nil {
		n.eventChan <- ev
	}
}
