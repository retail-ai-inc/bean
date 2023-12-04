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
	"sync"
	"time"

	"github.com/alphadose/haxmap"
)

type MemoryConfig struct {
	On        bool
	DelKeyAPI struct {
		EndPoint        string
		AuthBearerToken string
	}
}

// Memory stores arbitrary data with ttl.
type Memory struct {
	keys *haxmap.Map[string, Key]
	done chan struct{}
}

// A Key represents arbitrary data with ttl.
type Key struct {
	value any
	ttl   int64 // unix nano
}

// memoryDBConn is a singleton memory database connection.
var memoryDBConn *Memory
var memoryOnce sync.Once

// New creates a new memory that asynchronously cleans expired entries after the given ttl passes.
func NewMemory() *Memory {
	memoryOnce.Do(func() {

		// XXX: IMPORTANT - Run the ttl cleaning process in every 60 seconds.
		ttlCleaningInterval := 60 * time.Second

		h := haxmap.New[string, Key]()
		if h == nil {
			panic("failed to initialize the memory!")
		}

		memoryDBConn = &Memory{
			keys: h,
			done: make(chan struct{}),
		}

		go func() {
			ticker := time.NewTicker(ttlCleaningInterval)
			defer ticker.Stop()

			for {
				select {
				case <-ticker.C:
					now := time.Now().UnixNano()
					// O(N) iteration. It is linear time complexity.
					memoryDBConn.keys.ForEach(func(k string, item Key) bool {
						if item.ttl > 0 && now > item.ttl {
							memoryDBConn.keys.Del(k)
						}

						return true
					})

				case <-memoryDBConn.done:
					return
				}
			}
		}()
	})

	return memoryDBConn
}

// Get gets the value for the given key.
func (mem *Memory) GetMemory(k string) (interface{}, bool) {
	key, exists := mem.keys.Get(k)
	if !exists {
		return nil, false
	}

	if key.ttl > 0 && time.Now().UnixNano() > key.ttl {
		return nil, false
	}

	return key.value, true
}

// Set sets a value for the given key with an expiration duration.
// If the duration is 0 or less, it will be stored forever.
func (mem *Memory) SetMemory(key string, value any, duration time.Duration) {
	var expires int64

	if duration > 0 {
		expires = time.Now().Add(duration).UnixNano()
	}

	mem.keys.Set(key, Key{
		value: value,
		ttl:   expires,
	})
}

// Del deletes the key and its value from the memory cache.
func (mem *Memory) DelMemory(key string) {
	mem.keys.Del(key)
}

// Close closes the memory cache and frees up resources.
func (mem *Memory) CloseMemory() {
	mem.done <- struct{}{}
}
