// Package pool предоставляет пул объектов с методом Reset.
package pool

import "sync"

// Pool — пул объектов одного типа T.
type Pool[T interface{ Reset() }] struct {
	pool sync.Pool
}

// New создаёт пул. new вызывается, если в пуле нет свободных объектов.
func New[T interface{ Reset() }](new func() T) *Pool[T] {
	p := &Pool[T]{}
	p.pool.New = func() any {
		return new()
	}
	return p
}

// Get возвращает объект из пула.
func (p *Pool[T]) Get() T {
	return p.pool.Get().(T)
}

// Put возвращает объект в пул после сброса его состояния.
func (p *Pool[T]) Put(x T) {
	x.Reset()
	p.pool.Put(x)
}
