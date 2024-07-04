package chaperone

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
)

func newNode[T Message](ctx context.Context, cancel context.CancelCauseFunc, name string, handler Handler[T]) *Node[T] {
	return &Node[T]{
		ctx:         ctx,
		cancel:      cancel,
		Name:        name,
		handler:     handler,
		workerPool:  make(map[string][]Worker[T]),
		inputChans:  make(map[string]chan *Envelope[T]),
		outputChans: make(map[string]OutMux[T]),
		devNull:     make(chan *Envelope[T]),
		eventChan:   make(chan *Event[T]),
	}
}

func (n *Node[T]) AddWorkers(channelName string, num int, name string) {
	if _, ok := n.inputChans[channelName]; !ok {
		panic(fmt.Sprintf("channel %s not found", channelName))
	}

	c, cancel := context.WithCancelCause(n.ctx)
	if n.workerPool == nil {
		n.workerPool = make(map[string][]Worker[T])
	}

	for i := 0; i < num; i++ {
		numWorker := atomic.AddUint64(&n.workerCounter, 1)
		worker := Worker[T]{
			name:    fmt.Sprintf("%s-%d", name, numWorker),
			handler: n.handler,
			ctx:     c,
			cancel:  cancel,
			channel: n.inputChans[channelName], // Associate worker with the specific channel
		}
		n.workerPool[channelName] = append(n.workerPool[channelName], worker)
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
	for _, workers := range n.workerPool {
		for _, worker := range workers {
			go n.startWorker(worker)
		}
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
		case env, ok := <-w.channel:
			if !ok {
				// Channel closed
				return
			}
			env.inChan = w.channel
			outChName, ev := w.handler.Handle(env.message)

			if ev != nil {
				fmt.Printf("Worker %s encountered error: %v\n", w.name, ev)
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
