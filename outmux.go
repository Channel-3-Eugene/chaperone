package chaperone

func NewOutMux[Out Message](name string) *OutMux[Out] {
	return &OutMux[Out]{
		Name:     name,
		OutChans: make(map[string]*Edge[Out]),
		GoChans:  make(map[string]*Edge[Out]),
	}
}

func (o *OutMux[Out]) AddChannel(edge *Edge[Out]) {
	// Create a new output channel and goroutine for the edge.
	outChan := edge
	goChan := &Edge[Out]{
		name:    "go-" + edge.name,
		Channel: make(chan *Envelope[Out], cap(edge.Channel)),
	}

	go func(in, out *Edge[Out]) {
		for env := range in.Channel {
			out.Channel <- env // Send the envelope to the output channel.
		}
		close(out.Channel) // Close the output channel when the input channel is closed.
	}(outChan, goChan)

	o.OutChans[outChan.name] = outChan
	o.GoChans[goChan.name] = goChan
}

func (o *OutMux[Out]) Send(env *Envelope[Out]) {
	for name, edge := range o.OutChans {
		if edge == nil {
			delete(o.OutChans, name)
			delete(o.GoChans, name)
			continue
		}
		edge.Channel <- env
	}
}
