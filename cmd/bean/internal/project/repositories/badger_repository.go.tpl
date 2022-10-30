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

type BadgerRepository interface {
	SetString(c context.Context, key string, val string) error
	SetBytes(c context.Context, key string, val []byte) error
	GetString(c context.Context, key string) (string, error)
	GetBytes(c context.Context, key string) ([]byte, error)
	GetJson(c context.Context, key string) (map[string]interface{}, error)
}

type badgerRepository struct {
	client *badger.DB
}

func NewBadgerRepository(client *badger.DB) *badgerRepository {
	return &badgerRepository{client: client}
}

// SetString saves a string key and it's value in string into badgerdb.
func (r *badgerRepository) SetString(c context.Context, key string, val string, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	err := dbdrivers.BadgerSetString(r.client, key, val, ttl)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// SetBytes saves a string key and it's value in bytes into badgerdb.
func (r *badgerRepository) SetBytes(c context.Context, key string, val []byte, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	err := dbdrivers.BadgerSetBytes(r.client, key, val, ttl)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// GetString returns a string val of associated key.
func (r *badgerRepository) GetString(c context.Context, key string) (string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	data, err := dbdrivers.BadgerGetString(r.client, key)
	if err != nil {
		return "", errors.WithStack(err)
	}

	return data, nil
}

// GetBytes returns a bytes val of associated key.
func (r *badgerRepository) GetBytes(c context.Context, key string) ([]byte, error) {
	finish := trace.Start(c, "db")
	defer finish()

	data, err := dbdrivers.BadgerGetBytes(r.client, key)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}

// GetJson returns a json representation of associated key.
func (r *badgerRepository) GetJson(c context.Context, key string) (map[string]interface{}, error) {
	finish := trace.Start(c, "db")
	defer finish()

	data, err := dbdrivers.BadgerGetJson(r.client, key)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return data, nil
}
