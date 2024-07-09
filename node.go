package chaperone

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
)

func NewNode[T Message](ctx context.Context, name string, handler Handler[T]) *Node[T] {
	c, cancel := context.WithCancelCause(ctx)
	return &Node[T]{
		ctx:         c,
		cancel:      cancel,
		Name:        name,
		Handler:     handler,
		WorkerPool:  make(map[string][]*Worker[T]),
		InputChans:  make(map[string]chan *Envelope[T]),
		OutputChans: make(map[string]*OutMux[T]),
		EventChan:   make(chan *Event[T]),
	}
}

func (n *Node[T]) AddWorkers(channelName string, num int, name string) {
	if _, ok := n.InputChans[channelName]; !ok {
		panic(fmt.Sprintf("channel %s not found", channelName))
	}

	if n.WorkerPool == nil {
		n.WorkerPool = make(map[string][]*Worker[T])
	}

	for i := 0; i < num; i++ {
		numWorker := atomic.AddUint64(&n.WorkerCounter, 1)
		worker := &Worker[T]{
			name:    fmt.Sprintf("%s-%s-%d", channelName, name, numWorker),
			handler: n.Handler,
			ctx:     n.ctx,
			channel: n.InputChans[channelName], // Associate worker with the specific channel
		}
		n.WorkerPool[channelName] = append(n.WorkerPool[channelName], worker)
	}
}

func (n *Node[T]) AddInputChannel(name string, ch chan *Envelope[T]) {
	n.InputChans[name] = ch
}

func (n *Node[T]) AddOutputChannel(muxName, channelName string, ch chan *Envelope[T]) {
	mux, ok := n.OutputChans[muxName]
	if !ok {
		n.OutputChans[muxName] = NewOutMux[T](muxName)
		mux = n.OutputChans[muxName]
	}
	mux.AddChannel(channelName, ch)
}

func (n *Node[T]) Start() {
	fmt.Printf("Starting node %s\n", n.Name)

	for _, workers := range n.WorkerPool {
		for _, worker := range workers {
			go n.startWorker(worker)
		}
	}
}

func (n *Node[T]) startWorker(w *Worker[T]) {
	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case *Envelope[T]:
				err := NewEvent(ErrorLevelCritical, fmt.Errorf("worker %s panicked", w.name), x)
				n.handleWorkerEvent(w, err, x)
			case runtime.Error:
				err := NewEvent[T](ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", w.name, x), nil)
				n.handleWorkerEvent(w, err, nil)
			default:
				err := NewEvent[T](ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", w.name, x), nil)
				n.handleWorkerEvent(w, err, nil)
			}
		}
	}()

	fmt.Printf("Starting worker %s\n", w.name)

	for {
		select {
		case <-w.ctx.Done():
			fmt.Printf("Stopping worker %s\n", w.name)
			return
		case env, ok := <-w.channel:
			if !ok {
				// Channel closed
				return
			}

			fmt.Printf("Worker %s received message %v\n", w.name, env)

			env.Receive(n, w.channel)

			err := w.handler.Handle(n.ctx, env)
			if err != nil {
				fmt.Printf("Worker %s encountered error: %v\n", w.name, err)
				newErr := NewEvent(ErrorLevelError, err, env)
				n.handleWorkerEvent(w, newErr, env)
			} else if mux := env.OutChan; mux != nil {
				mux.Send(env)
			} else {
				err := NewEvent(ErrorLevelError, fmt.Errorf("output channel not found"), env)
				n.handleWorkerEvent(w, err, env)
			}
		}
	}
}

func (n *Node[T]) StopWorkers() {
	for _, workers := range n.WorkerPool {
		for _, worker := range workers {
			worker.ctx.Done()
		}
	}
}

func (n *Node[T]) RestartWorkers() {
	n.StopWorkers()
	n.Start()
}

func (n *Node[T]) Stop(evt *Event[T]) {
	n.cancel(*evt)
}

func (n *Node[T]) Restart(evt *Event[T]) {
	n.Stop(evt)
	n.Start()
}

func (n *Node[T]) handleWorkerEvent(_ *Worker[T], ev *Event[T], env *Envelope[T]) {
	env.NumRetries--
	if env.NumRetries > 0 {
		select {
		case env.InChan <- env:
			fmt.Printf("Retrying message %v\n", env.Message.String())
		default:
			fmt.Printf("Retry channel full for message %v\n", env.Message.String())
		}
	} else {
		fmt.Printf("No retries left for message %v\n", env.Message)
		// TODO: deadletter
	}

	if ev.Level >= ErrorLevelError {
		// Send event to supervisor if error level is Error or higher and we have an event channel
		if n.EventChan != nil {
			n.EventChan <- ev
		}
	}
}
