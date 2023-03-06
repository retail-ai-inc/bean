{{ .Copyright }}
package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/dgraph-io/badger/v3"
	"github.com/pkg/errors"
	"github.com/retail-ai-inc/bean/dbdrivers"
	"github.com/retail-ai-inc/bean/trace"
)

type MemoryRepository interface {
	GetString(c context.Context, key string) (string, error)
	GetBytes(c context.Context, key string) ([]byte, error)
	GetJSON(c context.Context, key string, dst interface{}) (bool, error)
	SetString(c context.Context, key string, val string, ttl time.Duration) error
	SetBytes(c context.Context, key string, val []byte, ttl time.Duration) error
	SetJSON(c context.Context, key string, data interface{}, ttl time.Duration) error
}

type memoryRepository struct {
	client *badger.DB
}

func NewMemoryRepository(client *badger.DB) *memoryRepository {
	return &memoryRepository{client: client}
}

// GetString returns a string val of associated key from memory. If the key doesn't exist in memory
// then this function will return `nil` error with empty string.
func (r *memoryRepository) GetString(c context.Context, key string) (string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	data, err := dbdrivers.MemoryGetString(r.client, key)
	if err != nil {
		return "", err
	}

	return data, nil
}

// GetBytes returns a bytes val of associated key from memory. If the key doesn't exist in memory
// then this function will return `nil` error with empty byte slice.
func (r *memoryRepository) GetBytes(c context.Context, key string) ([]byte, error) {
	finish := trace.Start(c, "db")
	defer finish()

	data, err := dbdrivers.MemoryGetBytes(r.client, key)
	if err != nil {
		return nil, err
	}

	return data, nil
}

// GetJSON will give you a json representation of associated key from memory. Pass an appropriate
// JSON structure var in `dst`. If the key doesn't exist in memory then this function will return
// `nil` error and `bool` as false.
func (r *memoryRepository) GetJSON(c context.Context, key string, dst interface{}) (bool, error) {
	finish := trace.Start(c, "db")
	defer finish()

	data, err := dbdrivers.MemoryGetBytes(r.client, key)
	if err != nil {
		return false, err
	}

	// Key not found
	if len(data) == 0 {
		return false, nil
	}

	if err = json.Unmarshal(data, &dst); err != nil {
		return false, errors.WithStack(err)
	}

	return true, nil
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

// SetJSON saves a string key and it's value as bytes into memory.
func (r *memoryRepository) SetJSON(c context.Context, key string, data interface{}, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return errors.WithStack(err)
	}

	err = dbdrivers.MemorySetBytes(r.client, key, jsonBytes, ttl)
	if err != nil {
		return err
	}

	return nil
}
