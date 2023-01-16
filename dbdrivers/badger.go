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
	"encoding/json"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v3"
)

type BadgerConfig struct {
	Path     string
	InMemory bool
}

// badgerDBConn is a singleton connection.
var badgerDBConn *badger.DB
var badgerOnce sync.Once

// Initialize the Badger database.
func InitBadgerConn(config BadgerConfig) *badger.DB {
	return connectBadgerDB(config)
}

// connectBadgerDB returns the singleton badger connection
func connectBadgerDB(config BadgerConfig) *badger.DB {

	badgerOnce.Do(func() {
		// IMPORTANT: InMemory mode can only use with empty "" dir
		opt := badger.DefaultOptions(config.Path).WithInMemory(config.InMemory)
		opt.Logger = nil

		// Open the Badger database located in the opt directory.
		// It will be created if it doesn't exist.
		var err error

		badgerDBConn, err = badger.Open(opt)
		if err != nil {
			panic(err)
		}
	})

	return badgerDBConn
}

// BadgerSetString saves a string key value pair to badgerdb. If you supply `ttl` greater than 0
// then badger will save the key into the db for that many seconds. Once the TTL has elapsed,
// the key will no longer be retrievable and will be eligible for garbage collection. Pass `ttl` as 0
// if you want to keep the key forever into the db until the server restarted or crashed.
func BadgerSetString(client *badger.DB, key string, val string, ttl time.Duration) error {

	return client.Update(func(txn *badger.Txn) error {
		if ttl > 0 {
			e := badger.NewEntry([]byte(key), []byte(val)).WithTTL(ttl)
			return txn.SetEntry(e)
		}

		err := txn.Set([]byte(key), []byte(val))
		return err
	})
}

// BadgerSetBytes saves a string key and it's value in bytes into  badgerdb. If you supply `ttl` greater than 0
// then badger will save the key into the db for that many seconds. Once the TTL has elapsed,
// the key will no longer be retrievable and will be eligible for garbage collection. Pass `ttl` as 0
// if you want to keep the key forever into the db until the server restarted or crashed.
func BadgerSetBytes(client *badger.DB, key string, val []byte, ttl time.Duration) error {

	return client.Update(func(txn *badger.Txn) error {
		if ttl > 0 {
			e := badger.NewEntry([]byte(key), val).WithTTL(ttl)
			return txn.SetEntry(e)
		}

		err := txn.Set([]byte(key), val)
		return err
	})
}

// BadgerGetString returns a string val of associated key.
func BadgerGetString(client *badger.DB, key string) (string, error) {

	var data []byte

	err := client.View(func(txn *badger.Txn) error {

		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		data, err = item.ValueCopy(nil)

		return err
	})

	if err != nil {
		return "", err
	}

	return string(data), nil
}

// BadgerGetBytes returns a byte val of associated key.
func BadgerGetBytes(client *badger.DB, key string) ([]byte, error) {

	var data []byte

	err := client.View(func(txn *badger.Txn) error {

		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		data, err = item.ValueCopy(nil)

		return err
	})

	if err != nil {
		return nil, err
	}

	return data, nil
}

// BadgerGetJson returns a json representation of associated key.
func BadgerGetJson(client *badger.DB, key string) (map[string]interface{}, error) {

	var data map[string]interface{}

	err := client.View(func(txn *badger.Txn) error {

		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		s, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		if err = json.Unmarshal(s, &data); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return data, nil
}
