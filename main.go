package main

import (
	"fmt"
	"sync/atomic"
)

const N = 100

func AtomicStoreInt32Add() (v int32) {
	for i := 0; i < N; i++ {
		atomic.StoreInt32(&v, int32(i)+1)
	}
	return
}

func AtomicStoreInt32Inc() (v int32) {
	for i := 0; i < N; i++ {
		atomic.StoreInt32(&v, v+1)
	}
	return
}
func AtomicEmpty() (v int32) {
	for i := 0; i < N; i++ {
	}
	return
}

func AtomicLoadInt32(v int32) {
	for i := 0; i < N; i++ {
		_ = atomic.LoadInt32(&v)
	}
}
func AtomicStoreLoadInt32(v int32) {
	for i := 0; i < N; i++ {
		atomic.StoreInt32(&v, 1)
		_ = atomic.LoadInt32(&v)
	}
}

func main() {
	var v int32 = 0
	v += AtomicStoreInt32Add()
	v += AtomicStoreInt32Inc()
	AtomicLoadInt32(v)
	AtomicStoreLoadInt32(v)
	fmt.Println(v)
}
