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

package dbdrivers

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestGetSet(t *testing.T) {
	cycle := 100 * time.Millisecond
	m := New()

	m.Set("sticky", "forever", 0)
	m.Set("hello", "Hello", cycle/2)
	hello, found := m.Get("hello")

	if !found {
		t.FailNow()
	}

	if hello.(string) != "Hello" {
		t.FailNow()
	}

	time.Sleep(cycle / 2)

	_, found = m.Get("hello")

	if found {
		t.FailNow()
	}

	time.Sleep(cycle)

	_, found = m.Get("404")

	if found {
		t.FailNow()
	}

	_, found = m.Get("sticky")

	if !found {
		t.FailNow()
	}
}

func TestDelete(t *testing.T) {
	m := New()
	m.Set("hello", "Hello", time.Hour)
	_, found := m.Get("hello")

	if !found {
		t.FailNow()
	}

	m.Del("hello")

	_, found = m.Get("hello")

	if found {
		t.FailNow()
	}
}

func BenchmarkNew(b *testing.B) {
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			New()
		}
	})
}

func BenchmarkGet(b *testing.B) {
	m := New()
	m.Set("Hello", "World", 0)

	b.ReportAllocs()
	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.Get("Hello")
		}
	})
}

const (
	epochs uintptr = 1 << 12
)

func BenchmarkGetWithSet(b *testing.B) {
	m := New()

	var writer uintptr
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		// use 1 thread as writer
		if atomic.CompareAndSwapUintptr(&writer, 0, 1) {
			for pb.Next() {
				for i := uintptr(0); i < epochs; i++ {
					m.Set("Hello", "World", 0)
				}
			}
		} else {
			for pb.Next() {
				for i := uintptr(0); i < epochs; i++ {
					j, _ := m.Get("Hello")
					if j.(string) != "World" {
						b.Fail()
					}
				}
			}
		}
	})
}

func BenchmarkSet(b *testing.B) {
	m := New()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.Set("Hello", "World", 0)
		}
	})
}

func BenchmarkDel(b *testing.B) {
	m := New()

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.Del("Hello")
		}
	})
}
