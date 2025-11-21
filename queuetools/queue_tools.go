package queue

import "fmt"

type Queue[V any] interface {
	//Enqueue []V 需要注意的数量过多的话，会导致内存过多 建议一次5000～10000条元素推送，不宜过多
	Enqueue(v []V) error
	Dequeue() (V, error)
	//DequeueBatch 建议批量pop出数据，到本地处理，不然一个一个 pop 性能损耗过大
	DequeueBatch(count int) ([]V, error)
	Len() (int64, error)
}

type QueueFullError struct {
	name string
	size int64
}

func (e *QueueFullError) Error() string {
	return fmt.Sprintf("fly queue %s full size is %d ", e.name, e.size)
}
