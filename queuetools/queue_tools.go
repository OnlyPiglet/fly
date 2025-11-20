package queue

import "fmt"

type Queue[V any] interface {
	Enqueue(v []V) error
	Dequeue() (V, error)
	Len() (int64, error)
}

type QueueFullError struct {
	name string
	size int64
}

func (e *QueueFullError) Error() string {
	return fmt.Sprintf("fly queue %s full size is %d ", e.name, e.size)
}
