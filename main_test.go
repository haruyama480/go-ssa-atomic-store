package main

import (
	"sync/atomic"
	"testing"
)

func BenchmarkAtomicStoreInt32(b *testing.B) {
	var v int32
	for i := 0; i < b.N; i++ {
		atomic.StoreInt32(&v, 1)
	}
}

func BenchmarkAtomicStoreInt32Add(b *testing.B) {
	var v int32
	for i := 0; i < b.N; i++ {
		atomic.StoreInt32(&v, int32(i)+1)
	}
}

func BenchmarkAtomicStoreInt32Inc(b *testing.B) {
	var v int32
	for i := 0; i < b.N; i++ {
		atomic.StoreInt32(&v, v+1)
	}
}

func BenchmarkEmpty(b *testing.B) {
	for i := 0; i < b.N; i++ {
	}
}

func BenchmarkAtomicLoadInt32(b *testing.B) {
	var v int32
	for i := 0; i < b.N; i++ {
		_ = atomic.LoadInt32(&v)
	}
}
func BenchmarkAtomicStoreLoadInt32(b *testing.B) {
	var v int32
	for i := 0; i < b.N; i++ {
		atomic.StoreInt32(&v, 1)
		_ = atomic.LoadInt32(&v)
	}
}
