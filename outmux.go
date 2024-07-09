package chaperone

func NewOutMux[T Message](name string) *OutMux[T] {
	return &OutMux[T]{
		Name:     name,
		OutChans: make(map[string]chan *Envelope[T]),
		GoChans:  make(map[string]chan *Envelope[T]),
	}
}

func (o *OutMux[T]) AddChannel(name string, ch chan *Envelope[T]) {
	if _, ok := o.GoChans[name]; ok {
		return // If the channel already exists, do nothing.
	}
	o.OutChans[name] = make(chan *Envelope[T], cap(ch))
	o.GoChans[name] = ch

	go func(inChan, outChan chan *Envelope[T]) {
		for env := range inChan {
			outChan <- env // Send the envelope to the output channel.
		}
		close(outChan) // Close the output channel when the input channel is closed.
	}(o.OutChans[name], ch)
}

func (o *OutMux[T]) Send(env *Envelope[T]) {
	for _, ch := range o.OutChans {
		if ch == nil {
			delete(o.OutChans, o.Name)
			delete(o.GoChans, o.Name)
			continue
		}
		ch <- env
	}
}

func (o *OutMux[T]) Stop() {
	for k, ch := range o.OutChans {
		if ch == nil {
			continue
		}
		close(ch)
		delete(o.OutChans, k)
	}
}
