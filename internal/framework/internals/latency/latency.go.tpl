{{ .Copyright }}
package latency

import (
	"bytes"
	"encoding/binary"
	"time"

	"{{ .PkgName }}/framework/internals/global"

	"github.com/dgraph-io/badger/v3"
	"github.com/labstack/echo/v4"
)

// Entry is a simple object storing latency
// and timestamp for an api response.
type Entry struct {
	Latency   time.Duration
	Timestamp int64
}

// SetAPILatencyWithTTL will serialize the val into []byte and save it until TTL.
func SetAPILatencyWithTTL(c echo.Context, key string, val interface{}, ttl time.Duration) {

	db := global.DBConn.BadgerDB

	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, val)
	if err != nil {
		panic(err)
	}

	err = db.Update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(key), buf.Bytes()).WithTTL(ttl)
		err := txn.SetEntry(e)

		return err
	})

	if err != nil {
		panic(err)
	}
}

// GetAllAPILatency returns a map containing all the api request latency data.
func GetAllAPILatency(c echo.Context) map[string]Entry {

	db := global.DBConn.BadgerDB

	latencyEntries := make(map[string]Entry)

	err := db.View(func(txn *badger.Txn) error {

		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		// All api uri start with "/".
		prefix := []byte("/")
		for it.Seek(prefix); it.ValidForPrefix(prefix); it.Next() {

			item := it.Item()
			k := item.Key()

			err := item.Value(func(v []byte) error {

				var e Entry
				buf := bytes.NewBuffer(v)

				err := binary.Read(buf, binary.BigEndian, &e)
				if err != nil {
					return err
				}

				latencyEntries[string(k)] = e

				return nil
			})

			if err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		panic(err)
	}

	return latencyEntries
}
