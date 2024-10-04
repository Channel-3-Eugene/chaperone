package chaperone

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
)

func NewNode[In, Out Message](name string, handler, lbHandler EnvHandler) *Node[In, Out] {
	n := &Node[In, Out]{
		name:            name,
		Handler:         handler,
		LoopbackHandler: lbHandler,
		WorkerPool:      make(map[string][]*Worker),
		In:              make(map[string]MessageCarrier),
		Out:             NewOutMux(name + ":output"),
		Events:          nil,
	}
	loopbackName := fmt.Sprintf("%s-loopback", name)
	n.In["loopback"] = &Edge{
		name:    "loopback",
		channel: make(chan Message, 1000),
	}
	n.AddWorkers(n.In["loopback"], 1, loopbackName, lbHandler)
	return n
}

func (n *Node[In, Out]) Name() string {
	return n.name
}

func (n *Node[In, Out]) SetEvents(edge MessageCarrier) {
	n.Events = edge
}

func (n *Node[In, Out]) AddWorkers(edge MessageCarrier, num int, name string, handler EnvHandler) {
	if handler == nil {
		return
	}

	if _, ok := n.In[edge.Name()]; !ok {
		panic(fmt.Sprintf("channel %s not found", edge.Name()))
	}

	if n.WorkerPool == nil {
		n.WorkerPool = make(map[string][]*Worker)
	}

	for i := 0; i < num; i++ {
		numWorker := atomic.AddUint64(&n.WorkerCounter, 1)
		worker := &Worker{
			name:      fmt.Sprintf("%s-%d", name, numWorker),
			handler:   handler,
			listening: edge, // Associate worker with the specific channel
		}
		n.WorkerPool[edge.Name()] = append(n.WorkerPool[edge.Name()], worker)
	}
}

func (n *Node[In, Out]) GetHandler() EnvHandler {
	return n.Handler
}

func (n *Node[In, Out]) AddInput(edge MessageCarrier) {
	n.In[edge.Name()] = edge
}

func (n *Node[In, Out]) AddOutput(edge MessageCarrier) {
	n.Out.AddChannel(edge)
}

func (n *Node[In, Out]) Start(ctx context.Context) {
	ctx, cancel := context.WithCancel(ctx)
	n.ctx = ctx
	n.cancel = cancel

	// start the node handler

	// start the worker handlers in their goroutines
	if len(n.WorkerPool) > 0 {
		for _, workers := range n.WorkerPool {
			for _, worker := range workers {
				go n.startWorker(ctx, worker)
			}
		}
	} else {
		n.Handler.Start(ctx)
	}
}

func (n *Node[In, Out]) startWorker(ctx context.Context, w *Worker) {
	atomic.AddInt64(&n.RunningWorkers, 1)
	defer func() {
		atomic.AddInt64(&n.RunningWorkers, -1)
	}()

	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case *Envelope[In]:
				err := NewEvent(ErrorLevelCritical, fmt.Errorf("worker %s panicked", w.name), x)
				n.handleWorkerEvent(w, err, x)
			case runtime.Error:
				err := NewEvent(ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", w.name, x), nil)
				n.handleWorkerEvent(w, err, nil)
			default:
				err := NewEvent(ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", w.name, x), nil)
				n.handleWorkerEvent(w, err, nil)
			}
		}
	}()

	evt := w.handler.Start(ctx)

	if evt != nil {
		if e, ok := evt.(*Event); ok {
			fmt.Printf("Worker %s error: %s\n", w.name, evt.Error())
			n.handleWorkerEvent(w, e, nil)
		} else {
			newErr := NewEvent(ErrorLevelError, fmt.Errorf("worker %s returned invalid event", w.name), nil)
			fmt.Printf("Worker %s error: %s\n", w.name, newErr.Error())
			n.handleWorkerEvent(w, newErr, nil)
		}
	}

	if len(n.In) > 1 {
		for {
			select {
			case <-ctx.Done():
				return
			case env, ok := <-w.listening.GetChannel():
				if !ok {
					// Channel closed
					fmt.Printf("Worker %s channel closed\n", w.name)
					return
				}

				newEnv, err := w.handler.Handle(ctx, env)

				if err != nil {
					if evt, ok := err.(*Event); ok {
						e := env.(*Envelope[In])
						fmt.Printf("Worker %s error: %s\n", w.name, evt.Error())
						n.handleWorkerEvent(w, evt, e)
					} else {
						newErr := NewEvent(ErrorLevelError, err, nil)
						fmt.Printf("Worker %s error: %s\n", w.name, newErr.Error())
						n.handleWorkerEvent(w, newErr, nil)
					}
				} else {
					n.Out.Send(newEnv)
				}
			}
		}
	}
}

func (n *Node[In, Out]) StopWorkers() {
	// stop the worker handlers
	n.cancel()
}

func (n *Node[In, Out]) RestartWorkers() {
	n.StopWorkers()
	n.Start(n.ctx)
}

func (n *Node[In, Out]) Stop(evt *Event) {
	n.SendEvent(evt)
	n.cancel()

	// any other cleanup?
}

func (n *Node[In, Out]) Restart(evt *Event) {
	n.Stop(evt)
	n.Start(n.ctx)
}

func (n *Node[In, Out]) SendEvent(evt *Event) {
	if n.Events != nil {
		n.Events.Send(evt)
	}
}

func (n *Node[In, Out]) handleWorkerEvent(_ *Worker, evt *Event, env *Envelope[In]) {
	if env != nil {
		env.NumRetries--
		if env.NumRetries > 0 {
			// Try again?
			n.In["loopback"].GetChannel() <- env
		}
	}

	// Send event to supervisor
	n.SendEvent(evt)
}

func (n *Node[In, Out]) RunningWorkerCount() int {
	num := atomic.LoadInt64(&n.RunningWorkers)
	return int(num)
}
