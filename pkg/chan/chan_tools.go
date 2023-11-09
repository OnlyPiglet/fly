package chan_tools

import "sync"

type ChanTool[T any] struct {
	once sync.Once
	ch   *chan T
}

func NewChanTools[T any](ch *chan T) *ChanTool[T] {
	return &ChanTool[T]{
		once: sync.Once{},
		ch:   ch,
	}
}

func (ct *ChanTool[T]) SafeCloseCh(ch chan T) {
	ct.once.Do(func() {
		close(ch)
	})
}
