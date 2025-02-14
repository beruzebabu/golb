package main

import "sync"

type SyncCache[T any] struct {
	value      T
	cacheMutex sync.Mutex
}

func (cache *SyncCache[T]) Set(value T) {
	cache.cacheMutex.Lock()
	cache.value = value
	cache.cacheMutex.Unlock()
}

func (cache *SyncCache[T]) Get() T {
	cache.cacheMutex.Lock()
	defer cache.cacheMutex.Unlock()
	return cache.value
}
