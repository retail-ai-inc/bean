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
	"context"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/retail-ai-inc/bean/v2/internal/dbdrivers"
)

const masterID uint64 = 0

// MasterCache provides functions for master redis db.
type MasterCache interface {
	KeyExists(c context.Context, key string) (bool, error)
	Keys(c context.Context, pattern string) ([]string, error)
	TTL(c context.Context, key string) (time.Duration, error)
	SetString(c context.Context, key string, data string, ttl time.Duration) error
	GetString(c context.Context, key string) (string, error)
	SetJSON(c context.Context, key string, data interface{}, ttl time.Duration) error
	GetJSON(c context.Context, key string, dst interface{}) (bool, error)
	MSetJSON(c context.Context, keys []string, data []interface{}, ttl time.Duration) error
	MGetJSON(c context.Context, dst interface{}, keys ...string) error
	MGet(c context.Context, keys ...string) ([]interface{}, error)
	HSet(c context.Context, key string, args ...interface{}) error
	HGet(c context.Context, key string, field string) (string, error)
	HMGet(c context.Context, key string, fields ...string) ([]interface{}, error)
	HGetAll(c context.Context, key string) (map[string]string, error)
	HGets(c context.Context, keysWithFields map[string]string) (map[string]string, error)
	RPush(c context.Context, key string, valueList []string) error
	LRange(c context.Context, key string, start, stop int64) ([]string, error)
	SAdd(c context.Context, key string, elements interface{}) error
	SRem(c context.Context, key string, elements interface{}) error
	SMembers(c context.Context, key string) ([]string, error)
	SRandMemberN(c context.Context, key string, count int64) ([]string, error)
	SIsMember(c context.Context, key string, element interface{}) (bool, error)
	IncrementValue(c context.Context, key string) error
	DelKey(c context.Context, keys ...string) error
	Expire(c context.Context, key string, ttl time.Duration) error
	Pipeline() redis.Pipeliner
	Pipelined(c context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error)
	Eval(c context.Context, script *Script, keysAndArgs ...interface{}) (interface{}, error)
}

type masterCache struct {
	cache *tenantCache // cache is a tenantCache with only master redis db; map[masterID]*redis.Client
}

// NewMasterCache creates a new MasterCache.
// This assumes it is called after the (*Bean).InitDB() func and takes (bean.DBDeps).MasterRedisDB as input.
func NewMasterCache(master *dbdrivers.RedisDBConn, prefix string, opts ...MasterCacheOption) MasterCache {

	m := &masterCache{
		cache: &tenantCache{
			clients: map[uint64]*dbdrivers.RedisDBConn{
				masterID: master,
			},
			prefix:    prefix,
			operation: "master-cache", // by default
			sep:       "_",            // by default
		},
	}

	for _, opt := range opts {
		opt(m)
	}

	return m
}

type MasterCacheOption func(*masterCache)

// OptTraceMCOperation is an option to set an operation name for tracing in MasterCache.
// It overrides a default value as long as the given operation name is not empty.
func OptTraceMCOperation(operation string) MasterCacheOption {
	return func(m *masterCache) {
		if operation != "" {
			m.cache.operation = operation
		}
	}
}

// OptSepMC ...
func OptSepMC(sep string) MasterCacheOption {
	return func(m *masterCache) {
		m.cache.sep = sep
	}
}

func (m *masterCache) KeyExists(c context.Context, key string) (bool, error) {
	return m.cache.KeyExists(c, masterID, key)
}

func (m *masterCache) Keys(c context.Context, pattern string) ([]string, error) {
	return m.cache.Keys(c, masterID, pattern)
}

func (m *masterCache) TTL(c context.Context, key string) (time.Duration, error) {
	return m.cache.TTL(c, masterID, key)
}

func (m *masterCache) SetString(c context.Context, key string, data string, ttl time.Duration) error {
	return m.cache.SetString(c, masterID, key, data, ttl)
}

func (m *masterCache) GetString(c context.Context, key string) (string, error) {
	return m.cache.GetString(c, masterID, key)
}

func (m *masterCache) SetJSON(c context.Context, key string, data interface{}, ttl time.Duration) error {
	return m.cache.SetJSON(c, masterID, key, data, ttl)
}

func (m *masterCache) GetJSON(c context.Context, key string, dst interface{}) (bool, error) {
	return m.cache.GetJSON(c, masterID, key, dst)
}

func (m *masterCache) MSetJSON(c context.Context, keys []string, data []interface{}, ttl time.Duration) error {
	return m.cache.MSetJSON(c, masterID, keys, data, ttl)
}

func (m *masterCache) MGetJSON(c context.Context, dst interface{}, keys ...string) error {
	return m.cache.MGetJSON(c, masterID, dst, keys...)
}

func (m *masterCache) MGet(c context.Context, keys ...string) ([]interface{}, error) {
	return m.cache.MGet(c, masterID, keys...)
}

// HSet accepts args in following formats:
// "key1", "value1", "key2", "value2" (as comma separated values)
// []string{"key1", "value1", "key2", "value2"}
// map[string]interface{}{"key1": "value1", "key2": "value2"}
func (m *masterCache) HSet(c context.Context, key string, args ...interface{}) error {
	return m.cache.HSet(c, masterID, key, args...)
}

func (m *masterCache) HGet(c context.Context, key string, field string) (string, error) {
	return m.cache.HGet(c, masterID, key, field)
}

func (m *masterCache) HMGet(c context.Context, key string, fields ...string) ([]interface{}, error) {
	return m.cache.HMGet(c, masterID, key, fields...)
}

func (m *masterCache) HGetAll(c context.Context, key string) (map[string]string, error) {
	return m.cache.HGetAll(c, masterID, key)
}

func (m *masterCache) HGets(c context.Context, keysWithFields map[string]string) (map[string]string, error) {
	return m.cache.HGets(c, masterID, keysWithFields)
}

func (m *masterCache) RPush(c context.Context, key string, valueList []string) error {
	return m.cache.RPush(c, masterID, key, valueList)
}

func (m *masterCache) LRange(c context.Context, key string, start, stop int64) ([]string, error) {
	return m.cache.LRange(c, masterID, key, start, stop)
}

func (m *masterCache) SAdd(c context.Context, key string, elements interface{}) error {
	return m.cache.SAdd(c, masterID, key, elements)
}

func (m *masterCache) SRem(c context.Context, key string, elements interface{}) error {
	return m.cache.SRem(c, masterID, key, elements)
}

func (m *masterCache) SMembers(c context.Context, key string) ([]string, error) {
	return m.cache.SMembers(c, masterID, key)
}

func (m *masterCache) SRandMemberN(c context.Context, key string, count int64) ([]string, error) {
	return m.cache.SRandMemberN(c, masterID, key, count)
}

func (m *masterCache) SIsMember(c context.Context, key string, element interface{}) (bool, error) {
	return m.cache.SIsMember(c, masterID, key, element)
}

func (m *masterCache) IncrementValue(c context.Context, key string) error {
	return m.cache.IncrementValue(c, masterID, key)
}

func (m *masterCache) DelKey(c context.Context, keys ...string) error {
	return m.cache.DelKey(c, masterID, keys...)
}

func (m *masterCache) Expire(c context.Context, key string, ttl time.Duration) error {
	return m.cache.Expire(c, masterID, key, ttl)
}

func (m *masterCache) Pipeline() redis.Pipeliner {
	return m.cache.Pipeline(masterID)
}

func (m *masterCache) Pipelined(c context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	return m.cache.Pipelined(c, masterID, fn)
}

func (m *masterCache) Eval(c context.Context, script *Script, keysAndArgs ...interface{}) (interface{}, error) {
	return m.cache.Eval(c, masterID, script, keysAndArgs)
}
