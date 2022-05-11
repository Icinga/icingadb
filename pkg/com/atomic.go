package com

import "sync/atomic"

// Atomic is a type-safe wrapper around atomic.Value.
type Atomic[T any] struct {
	v atomic.Value
}

func (a *Atomic[T]) Load() (_ T, ok bool) {
	if v, ok := a.v.Load().(box[T]); ok {
		return v.v, true
	}

	return
}

func (a *Atomic[T]) Store(v T) {
	a.v.Store(box[T]{v})
}

func (a *Atomic[T]) Swap(new T) (old T, ok bool) {
	if old, ok := a.v.Swap(box[T]{new}).(box[T]); ok {
		return old.v, true
	}

	return
}

func (a *Atomic[T]) CompareAndSwap(old, new T) (swapped bool) {
	return a.v.CompareAndSwap(box[T]{old}, box[T]{new})
}

// box allows, for the case T is an interface, nil values and values of different specific types implementing T
// to be stored in Atomic[T]#v (bypassing atomic.Value#Store()'s policy) by wrapping it (into a non-interface).
type box[T any] struct {
	v T
}
