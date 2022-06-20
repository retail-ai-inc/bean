{{ .Copyright }})
package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/retail-ai-inc/bean/trace"
)

type RedisRepository interface {
	GetJSON(c context.Context, companyID uint64, key string, dst interface{}) (bool, error)
	SetJSON(c context.Context, companyID uint64, key string, data interface{}, ttl time.Duration) error
	GetString(c context.Context, companyID uint64, key string) (string, error)
	SetString(c context.Context, companyID uint64, key string, data string, ttl time.Duration) error
	MGet(c context.Context, companyID uint64, keys ...string) ([]interface{}, error)
    RPush(c context.Context, companyID uint64, key string, valueList []string) error
	LRange(c context.Context, companyID uint64, key string) error
	IncrementValue(c context.Context, companyID uint64, key string) error
	DelKey(c context.Context, companyID uint64, key string) error
	SAdd(c context.Context, companyID uint64, key string, elements interface{}) error
	SRem(c context.Context, companyID uint64, key string, elements interface{}) error
	SIsMember(c context.Context, companyID uint64, key string, element interface{}) (bool, error)
	SMembers(c context.Context, companyID uint64, key string) ([]string, error)
}

type redisRepository struct {
	clients     map[uint64]*redis.Client
	cachePrefix string
}

func NewRedisRepository(clients map[uint64]*redis.Client, cachePrefix string) *redisRepository {
	return &redisRepository{
		clients:     clients,
		cachePrefix: cachePrefix,
	}
}

func (r *redisRepository) GetJSON(c context.Context, companyID uint64, key string, dst interface{}) (bool, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key
	fmt.Println("prefixKey", prefixKey)
	jsonStr, err := r.clients[companyID].Get(c, prefixKey).Result()
	if err == redis.Nil {
		return false, nil

	} else if err != nil {
		return false, errors.WithStack(err)
	}

	if err := json.Unmarshal([]byte(jsonStr), &dst); err != nil {
		return false, errors.WithStack(err)
	}

	return true, nil
}

// ttl in seconds.
func (r *redisRepository) SetJSON(c context.Context, companyID uint64, key string, data interface{}, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return errors.WithStack(err)
	}

	prefixKey := r.cachePrefix + "_" + key
	if err := r.clients[companyID].Set(c, prefixKey, string(jsonBytes), ttl).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *redisRepository) GetString(c context.Context, companyID uint64, key string) (string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key

	data, err := r.clients[companyID].Get(c, prefixKey).Result()

	if err == redis.Nil {

		return "", nil

	} else if err != nil {

		return "", errors.WithStack(err)
	}

	return data, nil
}

func (r *redisRepository) SetString(c context.Context, companyID uint64, key string, data string, ttl time.Duration) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key

	if err := r.clients[companyID].Set(c, prefixKey, data, ttl).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *redisRepository) MGet(c context.Context, companyID uint64, keys ...string) ([]interface{}, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixedKeysSlice := []string{}
	for _, key := range keys {
		prefixKey := r.cachePrefix + "_" + key
		prefixedKeysSlice = append(prefixedKeysSlice, prefixKey)
	}

	result, err := r.clients[companyID].MGet(c, prefixedKeysSlice...).Result()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return result, nil
}

func (r *redisRepository) RPush(c context.Context, companyID uint64, key string, valueList []string) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key

	if err := r.clients[companyID].RPush(c, prefixKey, valueList).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *redisRepository) LRange(c context.Context, companyID uint64, key string) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key

	if err := r.clients[companyID].LRange(c, prefixKey, 0, -1).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *redisRepository) IncrementValue(c context.Context, companyID uint64, key string) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key

	if err := r.clients[companyID].Incr(c, prefixKey).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *redisRepository) DelKey(c context.Context, companyID uint64, key string) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key

	if err := r.clients[companyID].Del(c, prefixKey).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *redisRepository) SAdd(c context.Context, companyID uint64, key string, elements interface{}) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key

	if err := r.clients[companyID].SAdd(c, prefixKey, elements).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *redisRepository) SRem(c context.Context, companyID uint64, key string, elements interface{}) error {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key

	if err := r.clients[companyID].SRem(c, prefixKey, elements).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (r *redisRepository) SIsMember(c context.Context, companyID uint64, key string, element interface{}) (bool, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key

	result, err := r.clients[companyID].SIsMember(c, prefixKey, element).Result()
	if err != nil {
		return false, errors.WithStack(err)
	}

	return result, nil
}

func (r *redisRepository) SMembers(c context.Context, companyID uint64, key string) ([]string, error) {
	finish := trace.Start(c, "db")
	defer finish()

	prefixKey := r.cachePrefix + "_" + key

	result, err := r.clients[companyID].SMembers(c, prefixKey).Result()
	if err != nil {
		return []string{}, errors.WithStack(err)
	}

	return result, nil
}