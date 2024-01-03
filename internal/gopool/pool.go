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

package gopool

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/panjf2000/ants/v2"
)

var (
	poolsMu sync.RWMutex
	pools   = make(map[string]*ants.Pool)
)

// Register makes a goroutine pool available by the provided name.
// If Register is called twice with the same name or if pool is nil,
// it returns error.
func Register(name string, pool *ants.Pool) error {
	poolsMu.Lock()
	defer poolsMu.Unlock()

	if pool == nil {
		return errors.New("gopool: Register pool is nil")
	}

	if _, dup := pools[name]; dup {
		return errors.New("gopool: Register called twice for pool " + name)
	}

	pools[name] = pool
	return nil
}

func UnregisterAllPools() {
	poolsMu.Lock()
	defer poolsMu.Unlock()

	for _, pool := range pools {
		pool.Release()
	}

	pools = make(map[string]*ants.Pool)
}

// Pools returns a sorted list of the names of the registered pools.
func Pools() []string {
	poolsMu.RLock()
	defer poolsMu.RUnlock()

	list := make([]string, 0, len(pools))

	for name := range pools {
		list = append(list, name)
	}

	sort.Strings(list)
	return list
}

func GetPool(poolName string) (*ants.Pool, error) {
	poolsMu.RLock()
	pool, ok := pools[poolName]
	poolsMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("gopool: unknown pool name %q", poolName)
	}

	return pool, nil
}
