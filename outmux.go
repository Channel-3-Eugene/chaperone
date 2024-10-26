package chaperone

import "fmt"

func NewOutMux(name string) OutMux {
	return OutMux{
		Name:     name,
		OutChans: make(map[string]MessageCarrier),
		GoChans:  make(map[string]MessageCarrier),
	}
}

func (o *OutMux) AddChannel(edge MessageCarrier) {
	// Create a new output channel and goroutine for the edge.
	outChan := NewEdge("spur:"+edge.Name(), nil, nil, cap(edge.GetChannel()), 1) // Returns *Edge
	goChan := edge.(*Edge)

	go func(in, out chan Message) {
		fmt.Printf("OutMux %s starting\n", o.Name)
		for env := range in {
			out <- env // Send the envelope to the output channel.
		}
		close(out) // Close the output channel when the input channel is closed.
	}(outChan.GetChannel(), goChan.GetChannel())

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
