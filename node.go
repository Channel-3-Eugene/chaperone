package chaperone

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
)

func NewNode[In, Out Message](ctx context.Context, name string, handler EnvHandler) *Node[In, Out] {
	fmt.Printf("Creating node %s\n\n", name)
	c, cancel := context.WithCancel(ctx)
	n := &Node[In, Out]{
		ctx:        c,
		cancel:     cancel,
		name:       name,
		Handler:    handler,
		WorkerPool: make(map[string][]*Worker),
		In:         make(map[string]MessageCarrier),
		Out:        NewOutMux("out"),
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
			name:      fmt.Sprintf("%s-%s-%d", edge.Name(), name, numWorker),
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
	fmt.Printf("node: %#v\n", n)
	n.Out.AddChannel(edge)
}

func (n *Node[In, Out]) Start() {
	fmt.Printf("Starting node %s\n", n.Name)

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

	fmt.Printf("Starting worker %s\n", w.name)

	for {
		select {
		case <-w.ctx.Done():
			fmt.Printf("Stopping worker %s\n", w.name)
			return
		case env, ok := <-w.listening.GetChannel():
			if !ok {
				// Channel closed
				return
			}

			fmt.Printf("Worker %s received message %v\n", w.name, env)

			newEnv, err := w.handler.Handle(n.ctx, env)
			if err != nil {
				fmt.Printf("Worker %s encountered error: %v\n", w.name, err)
				newErr := NewEvent(ErrorLevelError, err, env)
				e, ok := newEnv.(*Envelope[In])
				if ok {
					n.handleWorkerEvent(w, newErr, e)
				}
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
		n.In["loopback"].GetChannel() <- env
	} else {
		fmt.Printf("No retries left for message %v\n", env.Message)
		// TODO: deadletter
	}

	if ev.Level() >= ErrorLevelError {
		// Send event to supervisor if error level is Error or higher and we have an event channel
		n.Events.GetChannel() <- ev
	}
}
