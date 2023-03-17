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

	"github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"
)

type MemoryConfig struct {
	On        bool
	Dir       string
	DelKeyAPI struct {
		EndPoint        string
		AuthBearerToken string
	}
}

// memoryDBConn is a singleton memory database connection.
var memoryDBConn *badger.DB
var memoryOnce sync.Once

// Initialize the Memory database.
func InitMemoryConn(config MemoryConfig) *badger.DB {
	return connectMemoryDB(config)
}

// connectMemoryDB returns the singleton memory database connection
func connectMemoryDB(config MemoryConfig) *badger.DB {

	memoryOnce.Do(func() {
		// IMPORTANT: InMemory mode can only use with empty "" dir
		opt := badger.DefaultOptions(config.Dir).WithInMemory(config.On)
		opt.Logger = nil

		// Open the Badger database located in the opt directory.
		// It will be created if it doesn't exist.
		var err error

		memoryDBConn, err = badger.Open(opt)
		if err != nil {
			panic(err)
		}
	})

	return memoryDBConn
}

// MemoryGetString returns a string val of associated key from memory.
// If the key doesn't exist in memory then this function will return
// `nil` error with empty string.
func MemoryGetString(client *badger.DB, key string) (string, error) {

	var data []byte

	err := client.View(func(txn *badger.Txn) error {

		item, err := txn.Get([]byte(key))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil
			}

			return err
		}

		data, err = item.ValueCopy(nil)

		return err
	})

	if err != nil {
		return "", errors.WithStack(err)
	}

	return string(data), nil
}

// MemoryGetBytes returns a byte val of associated key from memory.
// If the key doesn't exist in memory then this function will return
// `nil` error with empty byte slice.
func MemoryGetBytes(client *badger.DB, key string) ([]byte, error) {

	var data []byte

	err := client.View(func(txn *badger.Txn) error {

		item, err := txn.Get([]byte(key))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil
			}

			return err
		}

		data, err = item.ValueCopy(nil)

		return err
	})

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}

// MemorySetString saves a string key value pair into memory. If you supply `ttl` greater than 0
// then this will save the key into the memory for that many seconds. Once the TTL has elapsed,
// the key will no longer be retrievable and will be eligible for garbage collection. Pass `ttl` as 0
// if you want to keep the key forever into the db until the server restarted or crashed.
func MemorySetString(client *badger.DB, key string, val string, ttl time.Duration) error {

	err := client.Update(func(txn *badger.Txn) error {
		if ttl > 0 {
			e := badger.NewEntry([]byte(key), []byte(val)).WithTTL(ttl)
			return txn.SetEntry(e)
		}

		return txn.Set([]byte(key), []byte(val))
	})

	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// MemorySetBytes saves a string key and it's value in bytes into memory. If you supply `ttl` greater than 0
// then this will save the key into the memory for that many seconds. Once the TTL has elapsed,
// the key will no longer be retrievable and will be eligible for garbage collection. Pass `ttl` as 0
// if you want to keep the key forever into the db until the server restarted or crashed.
func MemorySetBytes(client *badger.DB, key string, val []byte, ttl time.Duration) error {

	err := client.Update(func(txn *badger.Txn) error {
		if ttl > 0 {
			e := badger.NewEntry([]byte(key), val).WithTTL(ttl)
			return txn.SetEntry(e)
		}

		return txn.Set([]byte(key), val)
	})

	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// MemoryDelKey will just delete a key from memory if it is exist.
func MemoryDelKey(client *badger.DB, key string) error {

	err := client.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(key))
	})

	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}
