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
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/retail-ai-inc/bean/trace"
)

type RedisRepository interface {
	GetJSON(c context.Context, tenantID uint64, key string, dst interface{}) (bool, error)
	GetString(c context.Context, tenantID uint64, key string) (string, error)
	MGetJson(c context.Context, tenantID uint64, dst interface{}, keys ...string) error
	MSetJSON(c context.Context, tenantID uint64, keys []string, data []interface{}, ttl time.Duration) error
	MGet(c context.Context, tenantID uint64, keys ...string) ([]interface{}, error)
	HGet(c context.Context, tenantID uint64, key string, field string) (string, error)
	HGets(c context.Context, tenantID uint64, keysWithFields map[string]string) (map[string]string, error)
	LRange(c context.Context, tenantID uint64, key string, start, stop int64) ([]string, error)
	SMembers(c context.Context, tenantID uint64, key string) ([]string, error)
	SIsMember(c context.Context, tenantID uint64, key string, element interface{}) (bool, error)
	SetJSON(c context.Context, tenantID uint64, key string, data interface{}, ttl time.Duration) error
	SetString(c context.Context, tenantID uint64, key string, data string, ttl time.Duration) error
	HSet(c context.Context, tenantID uint64, key string, field string, data interface{}, ttl time.Duration) error
	RPush(c context.Context, tenantID uint64, key string, valueList []string) error
	IncrementValue(c context.Context, tenantID uint64, key string) error
	SAdd(c context.Context, tenantID uint64, key string, elements interface{}) error
	SRem(c context.Context, tenantID uint64, key string, elements interface{}) error
	DelKey(c context.Context, tenantID uint64, keys ...string) error
	Expire(c context.Context, tenantID uint64, key string, ttl time.Duration) error
}

type redisRepository struct {
	clients     map[uint64]*RedisDBConn
	cachePrefix string
}

func NewRedisRepository(clients map[uint64]*RedisDBConn, cachePrefix string) *redisRepository {
	return &redisRepository{clients, cachePrefix}
}

func (r *redisRepository) GetJSON(c context.Context, tenantID uint64, key string, dst interface{}) (bool, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	jsonStr, err := r.clients[tenantID].GetString(c, prefixKey)
	if err != nil {
		return false, err // This `err` is actually returning stack trace.
	} else if jsonStr == "" {
		return false, nil
	}

	if err := json.Unmarshal([]byte(jsonStr), &dst); err != nil {
		return false, errors.WithStack(err)
	}

	return true, nil
}

func (r *redisRepository) GetString(c context.Context, tenantID uint64, key string) (string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].GetString(c, prefixKey)
}

func (r *redisRepository) MGet(c context.Context, tenantID uint64, keys ...string) ([]interface{}, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixedKeysSlice := make([]string, 0, len(keys))
	for _, key := range keys {
		prefixKey := r.cachePrefix + "_" + key
		prefixedKeysSlice = append(prefixedKeysSlice, prefixKey)
	}

	return r.clients[tenantID].MGet(c, prefixedKeysSlice...)
}

func (r *redisRepository) MGetJson(c context.Context, tenantID uint64, dst interface{}, keys ...string) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixedKeysSlice := make([]string, 0, len(keys))
	for _, key := range keys {
		prefixKey := r.cachePrefix + "_" + key
		prefixedKeysSlice = append(prefixedKeysSlice, prefixKey)
	}

	values, err := r.clients[tenantID].MGet(c, prefixedKeysSlice...)
	if err != nil {
		return errors.WithStack(err)
	}

	strValues := "["
	for _, v := range values {
		if v != nil {
			strValues += v.(string) + ","
		}
	}
	if len(strValues) > 1 {
		strValues = strValues[:len(strValues)-1]
	}
	strValues += "]"
	err = json.Unmarshal([]byte(strValues), &dst)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *redisRepository) HGet(c context.Context, tenantID uint64, key, field string) (string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].HGet(c, prefixKey, field)
}

func (r *redisRepository) HGets(c context.Context, tenantID uint64, keysWithFields map[string]string) (map[string]string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	var mappedKeyFieldValues = make(map[string]string)

	for key, field := range keysWithFields {
		prefixKey := r.cachePrefix + "_" + key
		mappedKeyFieldValues[prefixKey] = field
	}

	return r.clients[tenantID].HGets(c, mappedKeyFieldValues)
}

func (r *redisRepository) LRange(c context.Context, tenantID uint64, key string, start, stop int64) ([]string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].GetLRange(c, prefixKey, start, stop)
}

func (r *redisRepository) SMembers(c context.Context, tenantID uint64, key string) ([]string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].SMembers(c, prefixKey)
}

func (r *redisRepository) SIsMember(c context.Context, tenantID uint64, key string, element interface{}) (bool, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].SIsMember(c, prefixKey, element)
}

func (r *redisRepository) SetJSON(c context.Context, tenantID uint64, key string, data interface{}, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].SetJSON(c, prefixKey, data, ttl)
}

func (r *redisRepository) MSetJSON(c context.Context, tenantID uint64, keys []string, data []interface{}, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	ln := len(keys)
	if ln != len(data) {
		return errors.New("key and data length mismatch")
	}
	for i, key := range keys {
		keys[i] = r.cachePrefix + "_" + key
	}
	var values = make([]interface{}, 0, ln*2)
	for i, datum := range data {
		values = append(values, keys[i], datum)
	}

	err := r.clients[tenantID].MSetWithTTL(c, ttl, values)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *redisRepository) SetString(c context.Context, tenantID uint64, key string, data string, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].Set(c, prefixKey, data, ttl)
}

func (r *redisRepository) HSet(c context.Context, tenantID uint64, key string, field string, data interface{}, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].HSet(c, prefixKey, field, data, ttl)
}

func (r *redisRepository) RPush(c context.Context, tenantID uint64, key string, valueList []string) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].RPush(c, prefixKey, valueList)
}

func (r *redisRepository) IncrementValue(c context.Context, tenantID uint64, key string) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].IncrementValue(c, prefixKey)
}

func (r *redisRepository) SAdd(c context.Context, tenantID uint64, key string, elements interface{}) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].SAdd(c, prefixKey, elements)
}

func (r *redisRepository) SRem(c context.Context, tenantID uint64, key string, elements interface{}) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].SRem(c, prefixKey, elements)
}

func (r *redisRepository) DelKey(c context.Context, tenantID uint64, keys ...string) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKeys := make([]string, len(keys))
	for i, key := range keys {
		prefixKeys[i] = r.cachePrefix + "_" + key
	}

	return r.clients[tenantID].DelKey(c, prefixKeys...)
}

func (r *redisRepository) Expire(c context.Context, tenantID uint64, key string, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return r.clients[tenantID].ExpireKey(c, prefixKey, ttl)
}
