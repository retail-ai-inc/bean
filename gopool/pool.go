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
