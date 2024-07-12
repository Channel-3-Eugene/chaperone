package chaperone

func NewOutMux(name string) OutMux {
	return OutMux{
		Name:     name,
		OutChans: make(map[string]MessageCarrier),
		GoChans:  make(map[string]MessageCarrier),
	}
}

func (o *OutMux) AddChannel(edge MessageCarrier) {
	// Create a new output channel and goroutine for the edge.
	outChan := edge
	goChan := &Edge{
		name:    "go-" + edge.Name(),
		channel: make(chan Message, cap(edge.GetChannel())),
	}

	go func(in, out MessageCarrier) {
		for env := range in.GetChannel() {
			out.GetChannel() <- env // Send the envelope to the output channel.
		}
		close(out.GetChannel()) // Close the output channel when the input channel is closed.
	}(outChan, goChan)

	o.OutChans[outChan.Name()] = outChan
	o.GoChans[goChan.Name()] = goChan
}

func (o *OutMux) Send(env Message) {
	for name, edge := range o.OutChans {
		if edge == nil {
			delete(o.OutChans, name)
			delete(o.GoChans, name)
			continue
		}
		edge.Send(env)
	}
}
