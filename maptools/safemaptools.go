package maptools

import "sync"

type SafeMap[k comparable, v any] struct {
	c map[k]v
	l sync.RWMutex
}

func NewSafeMap[k comparable, v any](size int) *SafeMap[k, v] {
	return &SafeMap[k, v]{
		c: make(map[k]v, size),
		l: sync.RWMutex{},
	}
}

func (sm *SafeMap[k, v]) Get(key k) (v v) {
	sm.l.RLock()
	defer sm.l.RUnlock()
	return sm.c[key]
}

func (sm *SafeMap[k, v]) Set(key k, val v) {
	sm.l.Lock()
	defer sm.l.Unlock()
	sm.c[key] = val
}

func (sm *SafeMap[k, v]) Del(key k) {
	sm.l.Lock()
	defer sm.l.Unlock()
	delete(sm.c, key)
}
