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
	poolsMu     sync.RWMutex
	pools       = make(map[string]*ants.Pool)
	defaultPool = initDefaultPool()
)

// NewPool creates a goroutine pool with the specified size and blocking tasks limit.
func NewPool(size *int, blockAfter *int) (*ants.Pool, error) {

	poolSize := -1 // capacity of the pool is unlimited by default
	if size != nil {
		poolSize = *size
	}

	maxBlockingTasks := 0 // unlimited blocking tasks by default
	if blockAfter != nil {
		maxBlockingTasks = *blockAfter
	}

	pool, err := ants.NewPool(poolSize, ants.WithMaxBlockingTasks(maxBlockingTasks))
	if err != nil {
		return nil, fmt.Errorf("gopool: initialization failed: %w", err)
	}

	return pool, nil
}

// Register makes a goroutine pool available by the provided name.
// If Register is called twice with the same name or if pool is nil,
// it returns error.
func Register(name string, pool *ants.Pool) error {

	if name == "" {
		return errors.New("gopool: register pool name is empty")
	}

	if pool == nil {
		return errors.New("gopool: register pool is nil")
	}

	poolsMu.Lock()
	defer poolsMu.Unlock()

	if _, dup := pools[name]; dup {
		return fmt.Errorf("gopool: register pool already exists with name %q", name)
	}

	pools[name] = pool
	return nil
}

func UnregisterAllPools() error {
	poolsMu.Lock()
	defer poolsMu.Unlock()

	// Basically release the pool in non-blocking way,
	// which means it will release the pool immediately without waiting for the tasks to be finished.
	for _, pool := range pools {
		pool.Release()
	}
	if defaultPool != nil {
		defaultPool.Release()
	}

	pools = make(map[string]*ants.Pool) // Reset the pools

	return nil // Always return nil
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

// GetDefaultPool returns the default pool.
func GetDefaultPool() *ants.Pool {
	return defaultPool
}

func initDefaultPool() *ants.Pool {
	pool, err := ants.NewPool(-1)
	if err != nil {
		panic(fmt.Sprintf("gopool: default pool initialization failed: %v", err))
	}

	if pool == nil {
		panic("gopool: default pool is nil")
	}

	return pool
}
