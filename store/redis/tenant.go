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

package redis

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	stderrors "errors"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/retail-ai-inc/bean/v3/internal/dbdrivers"
	"github.com/retail-ai-inc/bean/v3/trace"
)

// TenantCache provides functions for tenant redis dbs
// You pass in tenantID to connect to the corresponding redis db.
type TenantCache interface {
	KeyExists(c context.Context, tenantID uint64, key string) (bool, error)
	Keys(c context.Context, tenantID uint64, pattern string) ([]string, error)
	TTL(c context.Context, tenantID uint64, key string) (time.Duration, error)
	SetString(c context.Context, tenantID uint64, key string, data string, ttl time.Duration) error
	SetNX(c context.Context, tenantID uint64, key string, data string, ttl time.Duration) (bool, error)
	GetString(c context.Context, tenantID uint64, key string) (string, error)
	SetJSON(c context.Context, tenantID uint64, key string, data interface{}, ttl time.Duration) error
	GetJSON(c context.Context, tenantID uint64, key string, dst interface{}) (bool, error)
	MSetJSON(c context.Context, tenantID uint64, keys []string, data []interface{}, ttl time.Duration) error
	MGetJSON(c context.Context, tenantID uint64, dst interface{}, keys ...string) error
	MGet(c context.Context, tenantID uint64, keys ...string) ([]interface{}, error)
	HSet(c context.Context, tenantID uint64, key string, args ...interface{}) error
	HGet(c context.Context, tenantID uint64, key string, field string) (string, error)
	HMGet(c context.Context, tenantID uint64, key string, fields ...string) ([]interface{}, error)
	HGetAll(c context.Context, tenantID uint64, key string) (map[string]string, error)
	HGets(c context.Context, tenantID uint64, keysWithFields map[string]string) (map[string]string, error)
	RPush(c context.Context, tenantID uint64, key string, valueList []string) error
	LRange(c context.Context, tenantID uint64, key string, start, stop int64) ([]string, error)
	SAdd(c context.Context, tenantID uint64, key string, elements interface{}) error
	SRem(c context.Context, tenantID uint64, key string, elements interface{}) error
	SMembers(c context.Context, tenantID uint64, key string) ([]string, error)
	SRandMemberN(c context.Context, tenantID uint64, key string, count int64) ([]string, error)
	SIsMember(c context.Context, tenantID uint64, key string, element interface{}) (bool, error)
	IncrementValue(c context.Context, tenantID uint64, key string) error
	DelKey(c context.Context, tenantID uint64, keys ...string) error
	Expire(c context.Context, tenantID uint64, key string, ttl time.Duration) error
	Pipeline(tenantID uint64) redis.Pipeliner
	Pipelined(c context.Context, tenantID uint64, fn func(redis.Pipeliner) error) ([]redis.Cmder, error)
	Eval(c context.Context, tenantID uint64, script string, keys []string, args ...interface{}) (interface{}, error)
	EvalSha(c context.Context, tenantID uint64, sha1 string, keys []string, args ...interface{}) (interface{}, error)
	Run(c context.Context, tenantID uint64, script *redis.Script, keys []string, args ...interface{}) (interface{}, error)
}

type tenantCache struct {
	clients   map[uint64]*dbdrivers.RedisDBConn // map[tenantID]*redis.Client
	prefix    string                            // prefix for all keys cocatenated with underscore "_"
	operation string                            // operation name for tracing
	sep       string                            // sep connector between the prefix and the key.
}

// NewTenantCache creates a new TenantCache if you enable tenant mode (`detabase.tenant.on` in config).
// This assumes it is called after the (*Bean).InitDB() func and takes (bean.DBDeps).TenantRedisDBs as input.
func NewTenantCache(tenants map[uint64]*dbdrivers.RedisDBConn, prefix string, opts ...TenantCacheOption) TenantCache {
	t := &tenantCache{
		clients:   tenants,
		prefix:    prefix,
		operation: "tenant-cache", // by default
		sep:       "_",            // by default
	}

	for _, opt := range opts {
		opt(t)
	}

	return t
}

// ErrTenantNotFound is returned when the tenant has no Redis connection.
var ErrTenantNotFound = stderrors.New("tenant not found")

// client returns the Redis connection of the tenant.
func (t *tenantCache) client(tenantID uint64) (*dbdrivers.RedisDBConn, error) {
	client, ok := t.clients[tenantID]
	if !ok {
		return nil, fmt.Errorf("%w: tenantID:%d", ErrTenantNotFound, tenantID)
	}
	return client, nil
}

// key returns the Redis key of a cache key: the key with the prefix of the cache, if any.
func (t *tenantCache) key(key string) string {
	if t.prefix == "" {
		return key
	}
	return t.prefix + t.sep + key
}

// keyList returns the Redis keys of the cache keys.
func (t *tenantCache) keyList(keys []string) []string {
	if t.prefix == "" {
		return keys
	}
	pks := make([]string, len(keys))
	for i, key := range keys {
		pks[i] = t.key(key)
	}
	return pks
}

type TenantCacheOption func(*tenantCache)

// OptTraceTCOperation is an option to set an operation name for tracing in TenantCache.
// It overrides a default value as long as the given operation name is not empty.
func OptTraceTCOperation(operation string) func(*tenantCache) {
	return func(t *tenantCache) {
		if operation != "" {
			t.operation = operation
		}
	}
}

// OptSepTC ...
func OptSepTC(sep string) func(*tenantCache) {
	return func(t *tenantCache) {
		t.sep = sep
	}
}

func (t *tenantCache) KeyExists(c context.Context, tenantID uint64, key string) (bool, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return false, err
	}
	key = t.key(key)

	return client.KeyExists(c, key)
}

func (t *tenantCache) Keys(c context.Context, tenantID uint64, pattern string) ([]string, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return nil, err
	}

	pattern = t.key(pattern)

	return client.Keys(c, pattern)
}

func (t *tenantCache) TTL(c context.Context, tenantID uint64, key string) (time.Duration, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return time.Duration(0), err
	}

	key = t.key(key)

	return client.TTL(c, key)
}

func (t *tenantCache) SetString(c context.Context, tenantID uint64, key string, data string, ttl time.Duration) error {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return err
	}
	key = t.key(key)

	return client.Set(c, key, data, ttl)
}

func (t *tenantCache) SetNX(c context.Context, tenantID uint64, key string, data string, ttl time.Duration) (bool, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return false, err
	}
	key = t.key(key)

	return client.SetNX(c, key, data, ttl)
}

func (t *tenantCache) GetString(c context.Context, tenantID uint64, key string) (string, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return "", err
	}

	key = t.key(key)

	return client.GetString(c, key)
}

func (t *tenantCache) SetJSON(c context.Context, tenantID uint64, key string, data interface{}, ttl time.Duration) error {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return err
	}

	key = t.key(key)

	return client.SetJSON(c, key, data, ttl)
}

func (t *tenantCache) GetJSON(c context.Context, tenantID uint64, key string, dst interface{}) (bool, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return false, err
	}

	key = t.key(key)

	jsonStr, err := client.GetString(c, key)
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

func (t *tenantCache) MSetJSON(c context.Context, tenantID uint64, keys []string, data []interface{}, ttl time.Duration) error {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return err
	}
	ln := len(keys)
	if ln != len(data) {
		return errors.New("key and data length mismatch")
	}
	values := make([]interface{}, 0, ln*2)

	for i, key := range keys {
		jsonBytes, err := json.Marshal(data[i])
		if err != nil {
			return errors.WithStack(err)
		}
		values = append(values, t.key(key), string(jsonBytes))
	}

	if err := client.MSetWithTTL(c, ttl, values); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (t *tenantCache) MGetJSON(c context.Context, tenantID uint64, dst interface{}, keys ...string) error {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return err
	}

	pks := t.keyList(keys)

	values, err := client.MGet(c, pks...)
	if err != nil {
		return errors.WithStack(err)
	}

	// Build a JSON array of the existing values (missing keys are skipped).
	var buf bytes.Buffer
	buf.WriteByte('[')
	for _, v := range values {
		str, ok := v.(string)
		if !ok {
			continue
		}
		if buf.Len() > 1 {
			buf.WriteByte(',')
		}
		buf.WriteString(str)
	}
	buf.WriteByte(']')
	err = json.Unmarshal(buf.Bytes(), &dst)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (t *tenantCache) MGet(c context.Context, tenantID uint64, keys ...string) ([]interface{}, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()
	client, err := t.client(tenantID)
	if err != nil {
		return nil, err
	}
	pks := t.keyList(keys)

	return client.MGet(c, pks...)
}

// HSet accepts args in following formats:
// "key1", "value1", "key2", "value2" (as comma separated values)
// []string{"key1", "value1", "key2", "value2"}
// map[string]interface{}{"key1": "value1", "key2": "value2"}
func (t *tenantCache) HSet(c context.Context, tenantID uint64, key string, args ...interface{}) error {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return err
	}

	key = t.key(key)

	return client.HSet(c, key, args...)
}

func (t *tenantCache) HGet(c context.Context, tenantID uint64, key, field string) (string, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return "", err
	}

	key = t.key(key)

	return client.HGet(c, key, field)
}

func (t *tenantCache) HMGet(c context.Context, tenantID uint64, key string, fields ...string) ([]interface{}, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return nil, err
	}
	key = t.key(key)

	return client.HMGet(c, key, fields...)
}

func (t *tenantCache) HGetAll(c context.Context, tenantID uint64, key string) (map[string]string, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return nil, err
	}

	key = t.key(key)

	return client.HGetAll(c, key)
}

func (t *tenantCache) HGets(c context.Context, tenantID uint64, keysWithFields map[string]string) (map[string]string, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return nil, err
	}

	pksWithFields := make(map[string]string, len(keysWithFields))
	keyOf := make(map[string]string, len(keysWithFields)) // Redis key -> cache key
	for key, field := range keysWithFields {
		pksWithFields[t.key(key)] = field
		keyOf[t.key(key)] = key
	}

	values, err := client.HGets(c, pksWithFields)
	if err != nil {
		return nil, err
	}
	// Return the values by cache key, as passed by the caller (they were keyed by the prefixed Redis key).
	result := make(map[string]string, len(values))
	for pk, v := range values {
		result[keyOf[pk]] = v
	}
	return result, nil
}

func (t *tenantCache) RPush(c context.Context, tenantID uint64, key string, valueList []string) error {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return err
	}

	key = t.key(key)

	return client.RPush(c, key, valueList)
}

func (t *tenantCache) LRange(c context.Context, tenantID uint64, key string, start, stop int64) ([]string, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return nil, err
	}

	key = t.key(key)

	return client.GetLRange(c, key, start, stop)
}

func (t *tenantCache) SAdd(c context.Context, tenantID uint64, key string, elements interface{}) error {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return err
	}
	key = t.key(key)

	return client.SAdd(c, key, elements)
}

func (t *tenantCache) SRem(c context.Context, tenantID uint64, key string, elements interface{}) error {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return err
	}
	key = t.key(key)

	return client.SRem(c, key, elements)
}

func (t *tenantCache) SMembers(c context.Context, tenantID uint64, key string) ([]string, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return nil, err
	}

	key = t.key(key)

	return client.SMembers(c, key)
}

func (t *tenantCache) SRandMemberN(c context.Context, tenantID uint64, key string, count int64) ([]string, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return nil, err
	}

	key = t.key(key)

	return client.SRandMemberN(c, key, count)
}

func (t *tenantCache) SIsMember(c context.Context, tenantID uint64, key string, element interface{}) (bool, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return false, err
	}
	key = t.key(key)

	return client.SIsMember(c, key, element)
}

func (t *tenantCache) IncrementValue(c context.Context, tenantID uint64, key string) error {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return err
	}

	key = t.key(key)

	return client.IncrementValue(c, key)
}

func (t *tenantCache) DelKey(c context.Context, tenantID uint64, keys ...string) error {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return err
	}

	pks := t.keyList(keys)

	return client.DelKey(c, pks...)
}

func (t *tenantCache) Expire(c context.Context, tenantID uint64, key string, ttl time.Duration) error {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return err
	}
	key = t.key(key)

	return client.ExpireKey(c, key, ttl)
}

// Pipeline returns a pipeline on the tenant's primary, nil if the tenant has no Redis connection. The keys of the
// pipeline's commands are not prefixed.
func (t *tenantCache) Pipeline(tenantID uint64) redis.Pipeliner {
	client, err := t.client(tenantID)
	if err != nil {
		return nil
	}
	return client.Pipeline()
}

func (t *tenantCache) Pipelined(c context.Context, tenantID uint64, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return nil, err
	}
	return client.Pipelined(c, fn)
}

func (t *tenantCache) Eval(c context.Context, tenantID uint64, script string, keys []string, args ...interface{}) (interface{}, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return nil, err
	}

	return client.Eval(c, script, t.keyList(keys), args...)
}

func (t *tenantCache) EvalSha(c context.Context, tenantID uint64, sha1 string, keys []string, args ...interface{}) (interface{}, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return nil, err
	}

	return client.EvalSha(c, sha1, t.keyList(keys), args...)
}

func (t *tenantCache) Run(c context.Context, tenantID uint64, script *redis.Script, keys []string, args ...interface{}) (interface{}, error) {
	c, finish := trace.StartSpan(c, t.operation)
	defer finish()

	client, err := t.client(tenantID)
	if err != nil {
		return nil, err
	}

	return client.Run(c, script, t.keyList(keys), args...)
}
