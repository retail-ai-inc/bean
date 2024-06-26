// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// go test -bench=. -benchmem -benchtime=4s -cpu 2
package memory

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetSet(t *testing.T) {
	cycle := 100 * time.Millisecond
	m := NewMemoryCache()

	m.SetMemory("sticky", "forever", 0)
	m.SetMemory("hello", "Hello", cycle/2)
	hello, found := m.GetMemory("hello")

	if !found {
		t.FailNow()
	}

	if hello.(string) != "Hello" {
		t.FailNow()
	}

	time.Sleep(cycle / 2)

	_, found = m.GetMemory("hello")

	if found {
		t.FailNow()
	}

	time.Sleep(cycle)

	_, found = m.GetMemory("404")

	if found {
		t.FailNow()
	}

	_, found = m.GetMemory("sticky")

	if !found {
		t.FailNow()
	}
}

func TestDelete(t *testing.T) {
	m := NewMemoryCache()
	m.SetMemory("hello", "Hello", time.Hour)
	_, found := m.GetMemory("hello")

	if !found {
		t.FailNow()
	}

	m.DelMemory("hello")

	_, found = m.GetMemory("hello")

	if found {
		t.FailNow()
	}
}

func TestDeleteWithWildCard(t *testing.T) {
	type keyVal struct {
		key   string
		value interface{}
		ttl   time.Duration
	}
	type kvs struct {
		keyVal
		found bool
	}
	features := []keyVal{
		{key: "hello_world_1", value: "world_0", ttl: time.Hour},
		{key: "hello_world_2", value: "world_1", ttl: time.Hour},
		{key: "hello_world_3", value: "world_2", ttl: time.Hour},
		{key: "hello_1_world", value: "world_3", ttl: time.Hour},
		{key: "hello_2_world", value: "world_4", ttl: time.Hour},
		{key: "hello_3_world", value: "world_5", ttl: time.Hour},
		{key: "1_hello_world", value: "world_6", ttl: time.Hour},
		{key: "2_hello_world", value: "world_7", ttl: time.Hour},
		{key: "3_hello_world", value: "world_8", ttl: time.Hour},
	}

	tests := []struct {
		name     string
		kvs      []kvs
		wildcard string
	}{
		{name: "delete multi keys by tail wildcard", kvs: []kvs{
			{features[0], false}, {features[1], false}, {features[2], false},
			{features[3], true}, {features[4], true}, {features[5], true},
			{features[6], true}, {features[7], true}, {features[8], true},
		}, wildcard: "hello_world_*"},
		{name: "delete multi keys by middle wildcard", kvs: []kvs{
			{features[0], true}, {features[1], true}, {features[2], true},
			{features[3], false}, {features[4], false}, {features[5], false},
			{features[6], true}, {features[7], true}, {features[8], true},
		}, wildcard: "hello_*_world"},
		{name: "delete multi keys by head wildcard", kvs: []kvs{
			{features[0], true}, {features[1], true}, {features[2], true},
			{features[3], true}, {features[4], true}, {features[5], true},
			{features[6], false}, {features[7], false}, {features[8], false},
		}, wildcard: "*_hello_world"},
		{name: "delete multi keys by exact wildcard", kvs: []kvs{
			{features[0], false}, {features[1], false}, {features[2], false},
			{features[3], false}, {features[4], false}, {features[5], false},
			{features[6], false}, {features[7], false}, {features[8], false},
		}, wildcard: "*"},
		{name: "delete multi keys by two intermittent wildcards", kvs: []kvs{
			{features[0], false}, {features[1], false}, {features[2], false},
			{features[3], false}, {features[4], false}, {features[5], false},
			{features[6], false}, {features[7], false}, {features[8], false},
		}, wildcard: "*_*"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMemoryCache()

			for _, kv := range tt.kvs {
				m.SetMemory(kv.key, kv.value, kv.ttl)
			}

			m.DelMemory(tt.wildcard)

			for _, kv := range tt.kvs {
				_, found := m.GetMemory(kv.key)
				if found != kv.found {
					t.Errorf("key %s, expected found %v, got %v", kv.key, kv.found, found)
				}
			}
		})
	}
}

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			NewMemoryCache()
		}
	})
}

func BenchmarkGet(b *testing.B) {
	m := NewMemoryCache()
	m.SetMemory("Hello", "World", 0)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.GetMemory("Hello")
		}
	})
}

const (
	epochs uintptr = 1 << 12
)

func BenchmarkGetWithSet(b *testing.B) {
	m := NewMemoryCache()

	var writer uintptr
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// use 1 thread as writer
		if atomic.CompareAndSwapUintptr(&writer, 0, 1) {
			for pb.Next() {
				for i := uintptr(0); i < epochs; i++ {
					m.SetMemory("Hello", "World", 0)
				}
			}
		} else {
			for pb.Next() {
				for i := uintptr(0); i < epochs; i++ {
					j, _ := m.GetMemory("Hello")
					if j.(string) != "World" {
						b.Fail()
					}
				}
			}
		}
	})
}

func BenchmarkSet(b *testing.B) {
	m := NewMemoryCache()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.SetMemory("Hello", "World", 0)
		}
	})
}

func BenchmarkDel(b *testing.B) {
	m := NewMemoryCache()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.DelMemory("Hello")
		}
	})
}

// WARN: It takes about 30s to complete this benchmark with 1s `benchtime`.
func BenchmarkDeleteWithWildCard(b *testing.B) {

	setupKeys := func(m Cache, size int) {
		for i := 0; i < size; i++ {
			key := fmt.Sprintf("hello_%d", i)
			m.SetMemory(key, fmt.Sprintf("world_%d", i), time.Hour)
		}
	}

	sizes := []int{100, 1000, 10000}
	for _, size := range sizes {
		b.Run(fmt.Sprintf("delete %d keys", size), func(b *testing.B) {
			m := NewMemoryCache()
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				b.StopTimer()
				setupKeys(m, size)
				b.StartTimer()

				m.DelMemory("hello_*")
			}
		})
	}
}
