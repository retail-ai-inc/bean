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
	GetJSON(c context.Context, key string, dst interface{}) (bool, error)
	GetString(c context.Context, key string) (string, error)
	MGet(c context.Context, keys ...string) ([]interface{}, error)
	HGet(c context.Context, key string, field string) (string, error)
	LRange(c context.Context, key string, start, stop int64) ([]string, error)
	SMembers(c context.Context, key string) ([]string, error)
	SIsMember(c context.Context, key string, element interface{}) (bool, error)
	SetJSON(c context.Context, key string, data interface{}, ttl time.Duration) error
	SetString(c context.Context, key string, data string, ttl time.Duration) error
	HSet(c context.Context, key string, field string, data interface{}, ttl time.Duration) error
	RPush(c context.Context, key string, valueList []string) error
	IncrementValue(c context.Context, key string) error
	SAdd(c context.Context, key string, elements interface{}) error
	SRem(c context.Context, key string, elements interface{}) error
	DelKey(c context.Context, keys ...string) error
	Expire(c context.Context, key string, ttl time.Duration) error
	Pipelined(c context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error)
}

type masterCache struct {
	cache TenantCache
}

// NewMasterCache creates a new MasterCache.
// This assumes it is called after the (*Bean).InitDB() func and takes (bean.DBDeps).MasterRedisDB as input.
func NewMasterCache(master *dbdrivers.RedisDBConn, prefix string) MasterCache {

	if master == nil {
		panic("master redis db is not initialized properly")
	}

	return &masterCache{
		cache: &tenantCache{
			clients: map[uint64]*dbdrivers.RedisDBConn{
				masterID: master,
			},
			prefix: prefix,
		},
	}
}

func (m *masterCache) GetJSON(c context.Context, key string, dst interface{}) (bool, error) {
	return m.cache.GetJSON(c, masterID, key, dst)
}

func (m *masterCache) GetString(c context.Context, key string) (string, error) {
	return m.cache.GetString(c, masterID, key)
}

func (m *masterCache) MGet(c context.Context, keys ...string) ([]interface{}, error) {
	return m.cache.MGet(c, masterID, keys...)
}

func (m *masterCache) HGet(c context.Context, key string, field string) (string, error) {
	return m.cache.HGet(c, masterID, key, field)
}

func (m *masterCache) LRange(c context.Context, key string, start, stop int64) ([]string, error) {
	return m.cache.LRange(c, masterID, key, start, stop)
}

func (m *masterCache) SMembers(c context.Context, key string) ([]string, error) {
	return m.cache.SMembers(c, masterID, key)
}

func (m *masterCache) SIsMember(c context.Context, key string, element interface{}) (bool, error) {
	return m.cache.SIsMember(c, masterID, key, element)
}

func (m *masterCache) SetJSON(c context.Context, key string, data interface{}, ttl time.Duration) error {
	return m.cache.SetJSON(c, masterID, key, data, ttl)
}

func (m *masterCache) SetString(c context.Context, key string, data string, ttl time.Duration) error {
	return m.cache.SetString(c, masterID, key, data, ttl)
}

func (m *masterCache) HSet(c context.Context, key string, field string, data interface{}, ttl time.Duration) error {
	return m.cache.HSet(c, masterID, key, field, data, ttl)
}

func (m *masterCache) RPush(c context.Context, key string, valueList []string) error {
	return m.cache.RPush(c, masterID, key, valueList)
}

func (m *masterCache) IncrementValue(c context.Context, key string) error {
	return m.cache.IncrementValue(c, masterID, key)
}

func (m *masterCache) SAdd(c context.Context, key string, elements interface{}) error {
	return m.cache.SAdd(c, masterID, key, elements)
}

func (m *masterCache) SRem(c context.Context, key string, elements interface{}) error {
	return m.cache.SRem(c, masterID, key, elements)
}

func (m *masterCache) DelKey(c context.Context, keys ...string) error {
	return m.cache.DelKey(c, masterID, keys...)
}

func (m *masterCache) Expire(c context.Context, key string, ttl time.Duration) error {
	return m.cache.Expire(c, masterID, key, ttl)
}

func (m *masterCache) Pipelined(c context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	return m.cache.Pipelined(c, masterID, fn)
}
