package chaperone

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
)

func NewNode[In, Out Message](ctx context.Context, name string, handler EnvHandler) *Node[In, Out] {
	c, cancel := context.WithCancel(ctx)
	n := &Node[In, Out]{
		ctx:        c,
		cancel:     cancel,
		name:       name,
		Handler:    handler,
		WorkerPool: make(map[string][]*Worker),
		In:         make(map[string]MessageCarrier),
		Out:        NewOutMux(name + ":output"),
		Events:     nil,
	}
	n.In["loopback"] = &Edge{
		name:    "loopback",
		channel: make(chan Message, 1000),
	}
	n.AddWorkers(n.In["loopback"], 1, "loopback")
	return n
}

func (n *Node[In, Out]) Name() string {
	return n.name
}

func (n *Node[In, Out]) SetEvents(edge MessageCarrier) {
	n.Events = edge
}

func (n *Node[In, Out]) AddWorkers(edge MessageCarrier, num int, name string) {
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
			handler:   n.Handler,
			ctx:       n.ctx,
			listening: edge, // Associate worker with the specific channel
		}
		n.WorkerPool[edge.Name()] = append(n.WorkerPool[edge.Name()], worker)
	}
}

func (n *Node[In, Out]) AddInput(name string, edge MessageCarrier) {
	n.In[name] = edge
}

func (n *Node[In, Out]) AddOutput(edge MessageCarrier) {
	n.Out.AddChannel(edge)
}

func (n *Node[In, Out]) Start() {

	for _, workers := range n.WorkerPool {
		for _, worker := range workers {
			go n.startWorker(worker)
		}
	}
}

func (n *Node[In, Out]) startWorker(w *Worker) {
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

	for {
		select {
		case <-w.ctx.Done():
			return
		case env, ok := <-w.listening.GetChannel():
			if !ok {
				// Channel closed
				return
			}

			newEnv, err := w.handler.Handle(n.ctx, env)

			if err != nil {
				newErr := NewEvent(ErrorLevelError, err, env)
				e := env.(*Envelope[In])
				n.handleWorkerEvent(w, newErr, e)
			} else {
				n.Out.Send(newEnv)
			}
		}
	}
}

func (n *Node[In, Out]) StopWorkers() {
	for _, workers := range n.WorkerPool {
		for _, worker := range workers {
			worker.ctx.Done()
		}
	}
}

func (n *Node[In, Out]) RestartWorkers() {
	n.StopWorkers()
	n.Start()
}

func (n *Node[In, Out]) Stop(evt *Event) {
	n.cancel()
}

func (n *Node[In, Out]) Restart(evt *Event) {
	n.Stop(evt)
	n.Start()
}

func (n *Node[In, Out]) handleWorkerEvent(_ *Worker, ev *Event, env *Envelope[In]) {
	env.NumRetries--
	if env.NumRetries > 0 {
		// Try again?
		n.In["loopback"].GetChannel() <- env
	} else {
		// Send event to supervisor
		n.Events.GetChannel() <- ev
	}
}
