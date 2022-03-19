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

	"github.com/dgraph-io/badger/v3"
)

type BadgerConfig struct {
	Path     string
	InMemory bool
}

// Badger is a singleton connection.
var badgerConn *badger.DB
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

		badgerConn, err = badger.Open(opt)
		if err != nil {
			panic(err)
		}
	})

	return badgerConn
}

// SetString saves a string key value pair to badgerdb.
func SetString(key string, val string) error {

	return badgerConn.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte(key), []byte(val))
		return err
	})
}

// SetBytes saves a string key and it's value in bytes into badgerdb.
func SetBytes(key string, val []byte) error {

	return badgerConn.Update(func(txn *badger.Txn) error {
		err := txn.Set([]byte(key), val)
		return err
	})
}

// GetString returns a string val of associated key.
func GetString(key string) (string, error) {

	var data []byte

	err := badgerConn.View(func(txn *badger.Txn) error {

		item, err := txn.Get([]byte(key))
		if err != nil {
			return err
		}

		_, err = item.ValueCopy(data)

		return err
	})

	if err != nil {
		return "", err
	}

	return string(data), nil
}

// GetJson returns a json representation of associated key.
func GetJson(key string) (map[string]interface{}, error) {

	var data map[string]interface{}

	err := badgerConn.View(func(txn *badger.Txn) error {

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
