{{ .Copyright }}
package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/retail-ai-inc/bean/dbdrivers"
	"github.com/retail-ai-inc/bean/trace"
)

type RedisRepository interface {
	GetJSON(c context.Context, tenantID uint64, key string, dst interface{}) (bool, error)
	GetString(c context.Context, tenantID uint64, key string) (string, error)
	MGet(c context.Context, tenantID uint64, keys ...string) ([]interface{}, error)
	HGet(c context.Context, tenantID uint64, key string, field string) (string, error)
	HGets(c context.Context, tenantID uint64, keysWithFields map[string]string) (map[string]string, error)
	LRange(c context.Context, tenantID uint64, key string, start, stop int64) ([]string, error)
	SMembers(c context.Context, tenantID uint64, key string) ([]string, error)
	SIsMember(c context.Context, tenantID uint64, key string, element interface{}) (bool, error)
	SetJSON(c context.Context, tenantID uint64, key string, data interface{}, ttl time.Duration) error
	SetString(c context.Context, tenantID uint64, key string, data string, ttl time.Duration) error
	HSet(c context.Context, tenantID uint64, key string, field string, data interface{}) error
	RPush(c context.Context, tenantID uint64, key string, valueList []string) error
	IncrementValue(c context.Context, tenantID uint64, key string) error
	SAdd(c context.Context, tenantID uint64, key string, elements interface{}) error
	SRem(c context.Context, tenantID uint64, key string, elements interface{}) error
	DelKey(c context.Context, tenantID uint64, keys ...string) error
	Expire(c context.Context, tenantID uint64, key string, ttl time.Duration) error
}

type redisRepository struct {
	clients     map[uint64]*dbdrivers.RedisDBConn
	cachePrefix string
}

func NewRedisRepository(clients map[uint64]*dbdrivers.RedisDBConn, cachePrefix string) *redisRepository {
	return &redisRepository{clients, cachePrefix}
}

func (r *redisRepository) GetJSON(c context.Context, tenantID uint64, key string, dst interface{}) (bool, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	jsonStr, err := dbdrivers.RedisGetString(c, r.clients[tenantID], prefixKey)
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
	return dbdrivers.RedisGetString(c, r.clients[tenantID], prefixKey)
}

func (r *redisRepository) MGet(c context.Context, tenantID uint64, keys ...string) ([]interface{}, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixedKeysSlice := []string{}
	for _, key := range keys {
		prefixKey := r.cachePrefix + "_" + key
		prefixedKeysSlice = append(prefixedKeysSlice, prefixKey)
	}

	return dbdrivers.RedisMGet(c, r.clients[tenantID], prefixedKeysSlice...)
}

func (r *redisRepository) HGet(c context.Context, tenantID uint64, key, field string) (string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return dbdrivers.RedisHGet(c, r.clients[tenantID], prefixKey, field)
}

func (r *redisRepository) HGets(c context.Context, tenantID uint64, keysWithFields map[string]string) (map[string]string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	var mappedKeyFieldValues = make(map[string]string)

	for key, field := range keysWithFields {
		prefixKey := r.cachePrefix + "_" + key
		mappedKeyFieldValues[prefixKey] = field
	}

	return dbdrivers.RedisHgets(c, r.clients[tenantID], mappedKeyFieldValues)
}

func (r *redisRepository) LRange(c context.Context, tenantID uint64, key string, start, stop int64) ([]string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return dbdrivers.RedisGetLRange(c, r.clients[tenantID], prefixKey, start, stop)
}

func (r *redisRepository) SMembers(c context.Context, tenantID uint64, key string) ([]string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return dbdrivers.RedisSMembers(c, r.clients[tenantID], prefixKey)
}

func (r *redisRepository) SIsMember(c context.Context, tenantID uint64, key string, element interface{}) (bool, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return dbdrivers.RedisSIsMember(c, r.clients[tenantID], prefixKey, element)
}

func (r *redisRepository) SetJSON(c context.Context, tenantID uint64, key string, data interface{}, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return dbdrivers.RedisSetJSON(c, r.clients[tenantID], prefixKey, data, ttl)
}

func (r *redisRepository) SetString(c context.Context, tenantID uint64, key string, data string, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return dbdrivers.RedisSet(c, r.clients[tenantID], prefixKey, data, ttl)
}

func (r *redisRepository) HSet(c context.Context, tenantID uint64, key string, field string, data interface{}) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return dbdrivers.RedisHSet(c, r.clients[tenantID], prefixKey, field, data)
}

func (r *redisRepository) RPush(c context.Context, tenantID uint64, key string, valueList []string) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return dbdrivers.RedisRPush(c, r.clients[tenantID], prefixKey, valueList)
}

func (r *redisRepository) IncrementValue(c context.Context, tenantID uint64, key string) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return dbdrivers.RedisIncrementValue(c, r.clients[tenantID], prefixKey)
}

func (r *redisRepository) SAdd(c context.Context, tenantID uint64, key string, elements interface{}) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return dbdrivers.RedisSAdd(c, r.clients[tenantID], prefixKey, elements)
}

func (r *redisRepository) SRem(c context.Context, tenantID uint64, key string, elements interface{}) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return dbdrivers.RedisSRem(c, r.clients[tenantID], prefixKey, elements)
}

func (r *redisRepository) DelKey(c context.Context, tenantID uint64, keys ...string) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKeys := make([]string, len(keys))
	for i, key := range keys {
		prefixKeys[i] = r.cachePrefix + "_" + key
	}

	return dbdrivers.RedisDelKey(c, r.clients[tenantID], prefixKeys...)
}

func (r *redisRepository) Expire(c context.Context, tenantID uint64, key string, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	return dbdrivers.RedisExpireKey(c, r.clients[tenantID], prefixKey, ttl)
}