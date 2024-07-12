package chaperone

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
)

func NewNode[In, Out Message](ctx context.Context, name string, handler EnvHandler[In, Out]) *Node[In, Out] {
	c, cancel := context.WithCancel(ctx)
	n := &Node[In, Out]{
		ctx:        c,
		cancel:     cancel,
		Name:       name,
		Handler:    handler,
		WorkerPool: make(map[string][]*Worker[In, Out]),
		In:         make(map[string]*Edge[In]),
		Out:        NewOutMux[Out]("mux-" + name),
		Events:     nil,
	}
	n.In["loopback"] = &Edge[In]{
		name:    "loopback",
		Channel: make(chan *Envelope[In], 1000),
	}
	n.AddWorkers(n.In["loopback"], 1, "loopback")
	return n

}

func (n *Node[In, Out]) AddWorkers(edge *Edge[In], num int, name string) {
	if _, ok := n.In[edge.name]; !ok {
		panic(fmt.Sprintf("channel %s not found", edge.name))
	}

	if n.WorkerPool == nil {
		n.WorkerPool = make(map[string][]*Worker[In, Out])
	}

	for i := 0; i < num; i++ {
		numWorker := atomic.AddUint64(&n.WorkerCounter, 1)
		worker := &Worker[In, Out]{
			name:    fmt.Sprintf("%s-%s-%d", edge.name, name, numWorker),
			handler: n.Handler,
			ctx:     n.ctx,
			channel: edge, // Associate worker with the specific channel
		}
		n.WorkerPool[edge.name] = append(n.WorkerPool[edge.name], worker)
	}
}

func (n *Node[In, Out]) AddInput(name string, edge *Edge[In]) {
	n.In[name] = edge
}

func (n *Node[In, Out]) AddOutput(edge *Edge[Out]) {
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

func (n *Node[In, Out]) startWorker(w *Worker[In, Out]) {
	defer func() {
		if r := recover(); r != nil {
			switch x := r.(type) {
			case *Envelope[In]:
				err := NewEvent[In, Out](ErrorLevelCritical, fmt.Errorf("worker %s panicked", w.name), x)
				n.handleWorkerEvent(w, err, x)
			case runtime.Error:
				err := NewEvent[In, Out](ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", w.name, x), nil)
				n.handleWorkerEvent(w, err, nil)
			default:
				err := NewEvent[In, Out](ErrorLevelCritical, fmt.Errorf("worker %s panicked: %#v", w.name, x), nil)
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
		case env, ok := <-w.channel.Channel:
			if !ok {
				// Channel closed
				return
			}

			fmt.Printf("Worker %s received message %v\n", w.name, env)

			newEnv, err := w.handler.Handle(n.ctx, env)
			if err != nil {
				fmt.Printf("Worker %s encountered error: %v\n", w.name, err)
				newErr := NewEvent[In, Out](ErrorLevelError, err, env)
				n.handleWorkerEvent(w, newErr, env)
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

func (n *Node[In, Out]) Stop(evt *Event[In, Out]) {
	n.cancel()
}

func (n *Node[In, Out]) Restart(evt *Event[In, Out]) {
	n.Stop(evt)
	n.Start()
}

func (n *Node[In, Out]) handleWorkerEvent(_ *Worker[In, Out], ev *Event[In, Out], env *Envelope[In]) {
	env.NumRetries--
	if env.NumRetries > 0 {
		n.In["loopback"].Channel <- env
	} else {
		fmt.Printf("No retries left for message %v\n", env.Message)
		// TODO: deadletter
	}

	if ev.Level >= ErrorLevelError {
		// Send event to supervisor if error level is Error or higher and we have an event channel
		if n.Events != nil {
			n.Events <- ev
		}
	}
}
