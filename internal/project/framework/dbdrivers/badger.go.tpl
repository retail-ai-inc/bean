{{ .Copyright }}
package dbdrivers

import (
	"encoding/json"
	"sync"

	"github.com/dgraph-io/badger/v3"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
)

// Conn represents a badger db.
//type BadgerConn struct {
//	*badger.DB
//}

// Badger is a singleton connection.
var badgerConn *badger.DB
var badgerOnce sync.Once

// Initialize the Badger database.
func InitBadgerConn(e *echo.Echo) *badger.DB {
	return connectBadgerDB(e)
}

// connectBadgerDB returns the singleton badger connection
func connectBadgerDB(e *echo.Echo) *badger.DB {

	badgerOnce.Do(func() {

		// Iterate `badger` key and set `default` db here or set the first entry.
		config := "database.badger"
		badgerConfig := viper.GetStringMap(config)

		if len(badgerConfig) <= 0 {
			// Panic will be captured by `sentry` in staging and production.
			panic("Badger config not found")
		}

		dir := viper.GetString(config + ".dir")
		isInMem := viper.GetBool(config + ".inMemory")

		// XXX: IMPORTANT - InMemory mode can only use with empty "" dir
		opt := badger.DefaultOptions(dir).WithInMemory(isInMem)
		opt.Logger = nil

		// Open the Badger database located in the opt directory.
		// It will be created if it doesn't exist.
		var err error

		badgerConn, err = badger.Open(opt)
		if err != nil {
			e.Logger.Error(err)
			// Panic will be captured by `sentry` in staging and production.
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
