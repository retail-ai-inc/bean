{{ .Copyright }}
package repositories

import (
	"context"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"
	"github.com/retail-ai-inc/bean/dbdrivers"
	"github.com/retail-ai-inc/bean/trace"
)

type MemoryRepository interface {
	SetString(c context.Context, key string, val string, ttl time.Duration) error
	SetBytes(c context.Context, key string, val []byte, ttl time.Duration) error
	GetString(c context.Context, key string) (string, error)
	GetBytes(c context.Context, key string) ([]byte, error)
	GetJson(c context.Context, key string, dst interface{}) (bool, error)
}

type memoryRepository struct {
	client *badger.DB
}

func NewMemoryRepository(client *badger.DB) *memoryRepository {
	return &memoryRepository{client: client}
}

// SetString saves a string key and it's value as string into memory.
func (r *memoryRepository) SetString(c context.Context, key string, val string, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	err := dbdrivers.MemorySetString(r.client, key, val, ttl)
	if err != nil {
		return err
	}

	return nil
}

// SetBytes saves a string key and it's value as bytes into memory.
func (r *memoryRepository) SetBytes(c context.Context, key string, val []byte, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	err := dbdrivers.MemorySetBytes(r.client, key, val, ttl)
	if err != nil {
		return err
	}

	return nil
}

// GetString returns a string val of associated key from memory.
func (r *memoryRepository) GetString(c context.Context, key string) (string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	data, err := dbdrivers.MemoryGetString(r.client, key)
	if err != nil {
		return "", err
	}

	return data, nil
}

// GetBytes returns a bytes val of associated key from memory.
func (r *memoryRepository) GetBytes(c context.Context, key string) ([]byte, error) {
	finish := trace.Start(c, "db")
	defer finish()

	data, err := dbdrivers.MemoryGetBytes(r.client, key)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// GetJson will give you a json representation of associated key from memory. Pass an appropriate
// JSON structure var in `dst`.
func (r *memoryRepository) GetJson(c context.Context, key string, dst interface{}) (bool, error) {
	finish := trace.Start(c, "db")
	defer finish()

	data, err := dbdrivers.MemoryGetBytes(r.client, key)
	if err != nil {
		return false, err
	}

	if err = json.Unmarshal(data, &dst); err != nil {
		return false, errors.WithStack(err)
	}

	return true, nil
}
