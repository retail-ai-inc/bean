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
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/retail-ai-inc/bean/v2/internal/dbdrivers"
	"github.com/retail-ai-inc/bean/v2/trace"
)

// TenantCache provides functions for tenant redis dbs
// You pass in tenantID to connect to the corresponding redis db.
type TenantCache interface {
	KeyExists(c context.Context, tenantID uint64, key string) (bool, error)
	GetJSON(c context.Context, tenantID uint64, key string, dst interface{}) (bool, error)
	GetString(c context.Context, tenantID uint64, key string) (string, error)
	MGetJSON(c context.Context, tenantID uint64, dst interface{}, keys ...string) error
	MSetJSON(c context.Context, tenantID uint64, keys []string, data []interface{}, ttl time.Duration) error
	MGet(c context.Context, tenantID uint64, keys ...string) ([]interface{}, error)
	HGet(c context.Context, tenantID uint64, key string, field string) (string, error)
	HGets(c context.Context, tenantID uint64, keysWithFields map[string]string) (map[string]string, error)
	LRange(c context.Context, tenantID uint64, key string, start, stop int64) ([]string, error)
	SMembers(c context.Context, tenantID uint64, key string) ([]string, error)
	SRandMemberN(c context.Context, tenantID uint64, key string, count int64) ([]string, error)
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
	Pipelined(c context.Context, tenantID uint64, fn func(redis.Pipeliner) error) ([]redis.Cmder, error)
}

type tenantCache struct {
	clients map[uint64]*dbdrivers.RedisDBConn
	prefix  string
}

// NewTenantCache creates a new TenantCache if you enable tenant mode (`detabase.tenant.on` in config).
// This assumes it is called after the (*Bean).InitDB() func and takes (bean.DBDeps).TenantRedisDBs as input.
func NewTenantCache(tenants map[uint64]*dbdrivers.RedisDBConn, prefix string) TenantCache {

	if len(tenants) == 0 {
		panic("tenant mode is diable or tenant redis dbs are not initialized properly")
	}

	return &tenantCache{
		clients: tenants,
		prefix:  prefix,
	}
}

func (t *tenantCache) KeyExists(c context.Context, tenantID uint64, key string) (bool, error) {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].KeyExists(c, pk)
}

func (t *tenantCache) GetJSON(c context.Context, tenantID uint64, key string, dst interface{}) (bool, error) {

	pk := t.prefix + "_" + key
	jsonStr, err := t.clients[tenantID].GetString(c, pk)
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

func (t *tenantCache) GetString(c context.Context, tenantID uint64, key string) (string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].GetString(c, pk)
}

func (t *tenantCache) MGet(c context.Context, tenantID uint64, keys ...string) ([]interface{}, error) {
	finish := trace.Start(c, "db")
	defer finish()

	pks := make([]string, 0, len(keys))
	for _, key := range keys {
		pk := t.prefix + "_" + key
		pks = append(pks, pk)
	}
	return t.clients[tenantID].MGet(c, pks...)
}

func (t *tenantCache) MGetJSON(c context.Context, tenantID uint64, dst interface{}, keys ...string) error {
	finish := trace.Start(c, "db")
	defer finish()

	pks := make([]string, 0, len(keys))
	for _, key := range keys {
		pk := t.prefix + "_" + key
		pks = append(pks, pk)
	}

	values, err := t.clients[tenantID].MGet(c, pks...)
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

func (t *tenantCache) HGet(c context.Context, tenantID uint64, key, field string) (string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].HGet(c, pk, field)
}

func (t *tenantCache) HGets(c context.Context, tenantID uint64, keysWithFields map[string]string) (map[string]string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	var pksWithFields = make(map[string]string, len(keysWithFields))
	for key, field := range keysWithFields {
		pk := t.prefix + "_" + key
		pksWithFields[pk] = field
	}

	return t.clients[tenantID].HGets(c, pksWithFields)
}

func (t *tenantCache) LRange(c context.Context, tenantID uint64, key string, start, stop int64) ([]string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].GetLRange(c, pk, start, stop)
}

func (t *tenantCache) SMembers(c context.Context, tenantID uint64, key string) ([]string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].SMembers(c, pk)
}

func (t *tenantCache) SRandMemberN(c context.Context, tenantID uint64, key string, count int64) ([]string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].SRandMemberN(c, pk, count)
}

func (t *tenantCache) SIsMember(c context.Context, tenantID uint64, key string, element interface{}) (bool, error) {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].SIsMember(c, pk, element)
}

func (t *tenantCache) SetJSON(c context.Context, tenantID uint64, key string, data interface{}, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].SetJSON(c, pk, data, ttl)
}

func (t *tenantCache) MSetJSON(c context.Context, tenantID uint64, keys []string, data []interface{}, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	ln := len(keys)
	if ln != len(data) {
		return errors.New("key and data length mismatch")
	}
	values := make([]interface{}, 0, ln*2)
	for i := range keys {
		pk := t.prefix + "_" + keys[i]
		values = append(values, pk, data[i])
	}

	err := t.clients[tenantID].MSetWithTTL(c, ttl, values)
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (t *tenantCache) SetString(c context.Context, tenantID uint64, key string, data string, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].Set(c, pk, data, ttl)
}

func (t *tenantCache) HSet(c context.Context, tenantID uint64, key string, field string, data interface{}, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].HSet(c, pk, field, data, ttl)
}

func (t *tenantCache) RPush(c context.Context, tenantID uint64, key string, valueList []string) error {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].RPush(c, pk, valueList)
}

func (t *tenantCache) IncrementValue(c context.Context, tenantID uint64, key string) error {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].IncrementValue(c, pk)
}

func (t *tenantCache) SAdd(c context.Context, tenantID uint64, key string, elements interface{}) error {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].SAdd(c, pk, elements)
}

func (t *tenantCache) SRem(c context.Context, tenantID uint64, key string, elements interface{}) error {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].SRem(c, pk, elements)
}

func (t *tenantCache) DelKey(c context.Context, tenantID uint64, keys ...string) error {
	finish := trace.Start(c, "db")
	defer finish()

	pks := make([]string, len(keys))
	for i, key := range keys {
		pks[i] = t.prefix + "_" + key
	}

	return t.clients[tenantID].DelKey(c, pks...)
}

func (t *tenantCache) Expire(c context.Context, tenantID uint64, key string, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	pk := t.prefix + "_" + key
	return t.clients[tenantID].ExpireKey(c, pk, ttl)
}

func (t *tenantCache) Pipelined(c context.Context, tenantID uint64, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	finish := trace.Start(c, "db")
	defer finish()

	return t.clients[tenantID].Pipelined(c, fn)
}
