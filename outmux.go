package chaperone

func NewOutMux[T Message](name string) OutMux[T] {
	return OutMux[T]{
		Name:    name,
		inChans: make(map[string]chan *Envelope[T]),
		goChans: make(map[string]chan *Envelope[T]),
	}
}

func (o *OutMux[T]) AddChannel(name string, ch chan *Envelope[T]) {
	if _, ok := o.goChans[name]; ok {
		return // If the channel already exists, do nothing.
	}
	o.inChans[name] = make(chan *Envelope[T], cap(ch))
	o.goChans[name] = ch

	go func(inChan, outChan chan *Envelope[T]) {
		for env := range inChan {
			outChan <- env // Send the envelope to the output channel.
		}
		close(outChan) // Close the output channel when the input channel is closed.
	}(o.inChans[name], ch)
}

func (o *OutMux[T]) Send(env *Envelope[T]) {
	for _, ch := range o.inChans {
		if ch == nil {
			delete(o.inChans, o.Name)
			delete(o.goChans, o.Name)
			continue
		}
		ch <- env
	}
}

func (o *OutMux[T]) Stop() {
	for k, ch := range o.inChans {
		if ch == nil {
			continue
		}
		close(ch)
		delete(o.inChans, k)
	}
}
