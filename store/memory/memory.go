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

package memory

import (
	"strings"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
)

type Cache interface {
	GetMemory(key string) (interface{}, bool)
	SetMemory(key string, value any, duration time.Duration)
	DelMemory(key string)
	CloseMemory()
}

// memoryCache stores arbitrary data with ttl.
type memoryCache struct {
	keys      *haxmap.Map[string, data]
	done      chan struct{}
	closeOnce sync.Once
}

// data represents an arbitrary value with ttl.
type data struct {
	value any
	ttl   int64 // unix nano
}

// memoryDBConn is a singleton memory database connection.
var (
	memoryMu     sync.Mutex
	memoryDBConn *memoryCache
)

// NewMemoryCache New creates a new memory that asynchronously cleans expired entries after the given ttl passes.
// It returns the same cache to every caller until the cache is closed; after `CloseMemory`, a new cache is created.
func NewMemoryCache() Cache {
	memoryMu.Lock()
	defer memoryMu.Unlock()

	if memoryDBConn != nil && !memoryDBConn.closed() {
		return memoryDBConn
	}

	// XXX: IMPORTANT - Run the ttl cleaning process in every 60 seconds.
	ttlCleaningInterval := 60 * time.Second

	h := haxmap.New[string, data]()
	if h == nil {
		panic("failed to initialize the memory!")
	}

	mem := &memoryCache{
		keys: h,
		done: make(chan struct{}),
	}
	memoryDBConn = mem

	go func() {
		ticker := time.NewTicker(ttlCleaningInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				now := time.Now().UnixNano()
				// O(N) iteration. It is linear time complexity.
				mem.keys.ForEach(func(k string, d data) bool {
					if d.ttl > 0 && now > d.ttl {
						mem.keys.Del(k)
					}

					return true
				})

			case <-mem.done:
				return
			}
		}
	}()

	return mem
}

func (mem *memoryCache) closed() bool {
	select {
	case <-mem.done:
		return true
	default:
		return false
	}
}

// GetMemory Get gets the value for the given key.
func (mem *memoryCache) GetMemory(key string) (interface{}, bool) {
	d, exists := mem.keys.Get(key)
	if !exists {
		return nil, false
	}

	if d.ttl > 0 && time.Now().UnixNano() > d.ttl {
		return nil, false
	}

	return d.value, true
}

// SetMemory Set sets a value for the given key with an expiration duration.
// If the duration is 0 or less, it will be stored forever.
func (mem *memoryCache) SetMemory(key string, value any, duration time.Duration) {
	var expires int64

	if duration > 0 {
		expires = time.Now().Add(duration).UnixNano()
	}

	mem.keys.Set(key, data{
		value: value,
		ttl:   expires,
	})
}

// DelMemory Del deletes the key and its value from the memory cache.
// If the key has a wildcard (`*`), it will delete all keys that match the wildcard.
func (mem *memoryCache) DelMemory(key string) {

	if !strings.Contains(key, "*") {
		// Delete by a normal key.
		mem.keys.Del(key)
		return
	}

	// Delete by wildcard key.
	mem.keys.ForEach(func(k string, _ data) bool {
		if matchWildCard(k, key) {
			mem.keys.Del(k)
		}
		return true
	})
}

// matchWildCard reports whether str matches pattern, where `*` matches any sequence of characters (including
// none). It runs in O(len(str) x len(pattern)) at worst (the previous recursive version was exponential for patterns
// with several `*`). Matching bytes is equivalent to matching runes: `*` is ASCII and a literal UTF-8 sequence of
// the pattern can only match a whole character.
func matchWildCard(str, pattern string) bool {
	s, p := 0, 0
	star, starMatch := -1, 0 // position of the last `*` in pattern, and of what it matched up to in str
	for s < len(str) {
		switch {
		case p < len(pattern) && pattern[p] == '*':
			star, starMatch = p, s
			p++
		case p < len(pattern) && pattern[p] == str[s]:
			s++
			p++
		case star >= 0:
			// Backtrack: let the last `*` match one more character.
			starMatch++
			s, p = starMatch, star+1
		default:
			return false
		}
	}
	for p < len(pattern) && pattern[p] == '*' {
		p++
	}
	return p == len(pattern)
}

// CloseMemory Close closes the memory cache and frees up resources. It can be called more than once.
func (mem *memoryCache) CloseMemory() {
	mem.closeOnce.Do(func() { close(mem.done) })
}
