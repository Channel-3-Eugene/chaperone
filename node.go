package chaperone

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
)

func newNode[T Message](ctx context.Context, cancel context.CancelCauseFunc, name string, handler Handler[T]) *Node[T] {
	node := Node[T]{
		ctx:         ctx,
		cancel:      cancel,
		Name:        name,
		handler:     handler,
		workerPool:  []Worker[T]{},
		inputChans:  make(map[string]chan *Envelope[T]),
		outputChans: make(map[string]OutMux[T]),
		devNull:     NewChannel[T](10, true),
		eventChan:   nil, // Supervisor can attach to this
	}

	return &node
}

func (n *Node[T]) AddWorkers(num int, name string, handler Handler[T]) {
	c, cancel := context.WithCancelCause(n.ctx)
	for i := 0; i < num; i++ {
		numWorker := atomic.AddUint64(&n.workerCounter, 1)
		n.workerPool = append(n.workerPool, Worker[T]{
			name:    fmt.Sprintf("%s-%d", name, numWorker),
			handler: handler,
			ctx:     c,
			cancel:  cancel,
		})
	}
}

func (n *Node[T]) AddInputChannel(name string, ch chan *Envelope[T]) {
	n.inputChans[name] = ch
}

func (n *Node[T]) AddOutputChannel(name string, ch chan *Envelope[T]) {
	mux, ok := n.outputChans[name]
	if !ok {
		n.outputChans[name] = OutMux[T]{outChans: []chan *Envelope[T]{ch}}
		return
	}
	mux.outChans = append(mux.outChans, ch)
}

func (n *Node[T]) Start() {
	for _, worker := range n.workerPool {
		go n.startWorker(worker)
	}
}

func (n *Node[T]) startWorker(w Worker[T]) {
	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case *Envelope[T]:
				err := NewEvent(ErrorLevelCritical, fmt.Errorf("worker %s panicked", w.name), x.message)
				n.handleWorkerEvent(&w, &err, x)
			case runtime.Error:
				err := NewEvent[T](ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", w.name, x), nil)
				n.handleWorkerEvent(&w, &err, nil)
			default:
				err := NewEvent[T](ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", w.name, x), nil)
				n.handleWorkerEvent(&w, &err, nil)
			}
		}
	}()

	for {
		select {
		case <-w.ctx.Done():
			return
		default:
			for _, ch := range n.inputChans {
				select {
				case env, ok := <-ch:
					if !ok {
						return // Channel closed
					}

					env.inChan = ch
					outChName, ev := w.handler.Handle(env.message)

					if ev != nil {
						newErr := NewEvent(ErrorLevelError, ev, env.message)
						n.handleWorkerEvent(&w, &newErr, env)
					} else if mux, ok := n.outputChans[outChName]; ok {
						n.Send(env, mux)
					} else {
						newErr := NewEvent(ErrorLevelError, fmt.Errorf("output channel %s not found", outChName), env.message)
						n.handleWorkerEvent(&w, &newErr, env)
						n.devNull <- env
					}
				default:
					// No message to read, continue to next channel
					continue
				}
			}
		}
	}
}

func (n *Node[T]) Stop() {
	n.cancel(NewEvent[T](ErrorLevelInfo, fmt.Errorf("stopping node %s", n.Name), nil))
}

func (n *Node[T]) Send(env *Envelope[T], mux OutMux[T]) {
	for _, ch := range mux.outChans {
		select {
		case ch <- env:
			// fmt.Printf("Message send to %#v, len %d, Cap: %d\n", ch, len(ch), cap(ch))
		default:
			fmt.Printf("Channel %#v is full, message dropped\n", ch)
		}
	}
}

func (n *Node[T]) handleWorkerEvent(w *Worker[T], ev *Event[T], env *Envelope[T]) {
	env.numRetries--
	if env.numRetries > 0 {
		select {
		case env.inChan <- env:
			// fmt.Printf("Retrying message %v\n", env.message)
		default:
			fmt.Printf("Retry channel full for message %v\n", env.message)
		}
	} else {
		select {
		case n.devNull <- env:
			fmt.Printf("Sending message %v to devNull\n", env.message)
		default:
			fmt.Printf("devNull channel full for message %v\n", env.message)
		}
	}

	if ev.Level >= ErrorLevelError {
		select {
		case n.eventChan <- ev:
			// fmt.Printf("Event sent: %v\n", ev)
		default:
			fmt.Printf("Event channel full (cap %d), event dropped: %v\n", cap(n.eventChan), ev)
		}
		w.cancel(ev)
	}
}
