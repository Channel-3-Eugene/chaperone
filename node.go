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
		devNull:     NewChannel[T](1000, true),
		eventChan:   nil, // Supervisor can attach to this
	}

	return &node
}

func (n *Node[T]) AddWorkers(num int, name string) {
	c, cancel := context.WithCancelCause(n.ctx)
	for i := 0; i < num; i++ {
		numWorker := atomic.AddUint64(&n.workerCounter, 1)
		n.workerPool = append(n.workerPool, Worker[T]{
			name:    fmt.Sprintf("%s-%d", name, numWorker),
			handler: n.handler,
			ctx:     c,
			cancel:  cancel,
		})
	}
}

func (n *Node[T]) AddInputChannel(name string, ch chan *Envelope[T]) {
	n.inputChans[name] = ch
}

func (n *Node[T]) AddOutputChannel(muxName, channelName string, ch chan *Envelope[T]) {
	mux, ok := n.outputChans[muxName]
	if !ok {
		n.outputChans[muxName] = NewOutMux[T](muxName)
		mux = n.outputChans[muxName]
	}
	mux.AddChannel(channelName, ch)
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
		stop := false
		openChannels := len(n.inputChans)
		for _, ch := range n.inputChans {
			select {
			case <-w.ctx.Done():
				stop = true

			case env, ok := <-ch:
				if !ok {
					openChannels--
					if openChannels == 0 {
						fmt.Printf("All input channels closed for worker %s\n", w.name)
						continue
					}
				}

				env.inChan = ch
				outChName, ev := w.handler.Handle(env.message)

				if ev != nil {
					newErr := NewEvent(ErrorLevelError, ev, env.message)
					n.handleWorkerEvent(&w, &newErr, env)
				} else if mux, ok := n.outputChans[outChName]; ok {
					mux.Send(env)
				} else {
					newErr := NewEvent(ErrorLevelError, fmt.Errorf("output channel %s not found", outChName), env.message)
					n.handleWorkerEvent(&w, &newErr, env)
					n.devNull <- env
				}
			}

			if stop && openChannels == 0 {
				n.Stop()
				return
			}
		}
	}
}

func (n *Node[T]) Stop() {
	n.cancel(NewEvent[T](ErrorLevelInfo, fmt.Errorf("stopping node %s", n.Name), nil))
	for _, mux := range n.outputChans {
		mux.Stop()
	}
}

func (n *Node[T]) handleWorkerEvent(w *Worker[T], ev *Event[T], env *Envelope[T]) {
	env.numRetries--
	if env.numRetries > 0 {
		select {
		case env.inChan <- env:
			fmt.Printf("Retrying message %v\n", env.message)
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

	eventCount := 0
	if ev.Level >= ErrorLevelError {
		select {
		case n.eventChan <- ev:
			eventCount++
			fmt.Printf("eventCount: %d\r", eventCount)
			// success
		default:
			fmt.Printf("Event channel full (cap %d), event dropped: %v\n", cap(n.eventChan), ev)
		}
		w.cancel(ev)
	}
}
