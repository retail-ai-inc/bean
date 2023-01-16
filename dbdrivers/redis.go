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
	"math/rand"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"github.com/retail-ai-inc/bean/aes"
	"gorm.io/gorm"
)

// IMPORTANT: This structure is holding any kind of redis connection using a map in bean.go.
type RedisDBConn struct {
	Host *redis.Client
	Read map[uint64]*redis.Client
	Name int
}

type RedisConfig struct {
	Master *struct {
		Database int
		Password string
		Host     string
		Port     string
		Read     []string
	}
	Prefix             string
	Maxretries         int
	PoolSize           int
	MinIdleConnections int
	DialTimeout        time.Duration
	ReadTimeout        time.Duration
	WriteTimeout       time.Duration
	PoolTimeout        time.Duration
}

type KeyFieldPair struct {
	Key   string `json:"key"`
	Field string `json:"field"`
}

type FieldValuePair struct {
	Field string `json:"field"`
	Value string `json:"value"`
}

var cachePrefix string

func InitRedisTenantConns(config RedisConfig, master *gorm.DB, tenantAlterDbHostParam, tenantDBPassPhraseKey string) map[uint64]*RedisDBConn {
	cachePrefix = config.Prefix
	tenantCfgs := GetAllTenantCfgs(master)

	if len(tenantCfgs) > 0 {
		return getAllRedisTenantDB(config, tenantCfgs, tenantAlterDbHostParam, tenantDBPassPhraseKey)
	}

	return nil
}

func InitRedisMasterConn(config RedisConfig) map[uint64]*RedisDBConn {

	masterCfg := config.Master
	masterRedisDB := make(map[uint64]*RedisDBConn, 1)

	if masterCfg != nil {

		masterRedisDB[0] = &RedisDBConn{}

		masterRedisDB[0].Host, masterRedisDB[0].Name = connectRedisDB(
			masterCfg.Password, masterCfg.Host, masterCfg.Port, masterCfg.Database,
			config.Maxretries, config.PoolSize, config.MinIdleConnections, config.DialTimeout,
			config.ReadTimeout, config.WriteTimeout, config.PoolTimeout,
		)

		if len(masterCfg.Read) > 0 {
			redisReadConn := make(map[uint64]*redis.Client, len(masterCfg.Read))

			for i, readHost := range masterCfg.Read {
				var host, port string

				s := strings.Split(readHost, ":")
				host = s[0]
				if len(s) != 2 {
					port = masterCfg.Port
				} else {
					port = s[1]
				}

				redisReadConn[uint64(i)], _ = connectRedisDB(
					masterCfg.Password, host, port, masterCfg.Database,
					config.Maxretries, config.PoolSize, config.MinIdleConnections, config.DialTimeout,
					config.ReadTimeout, config.WriteTimeout, config.PoolTimeout,
				)

			}

			masterRedisDB[0].Read = redisReadConn
		}
	}

	return masterRedisDB
}

func RedisGetString(c context.Context, clients *RedisDBConn, key string) (string, error) {
	var err error
	var str string

	noOfReadReplica := len(clients.Read)

	// Check the read replicas are available or not.
	if noOfReadReplica == 1 {
		str, err = clients.Read[0].Get(c, key).Result()
		if err != nil {
			str, err = clients.Host.Get(c, key).Result()
		}
	} else if noOfReadReplica > 1 {
		// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
		rand.Seed(time.Now().UnixNano())
		readHost := rand.Intn(noOfReadReplica)

		str, err = clients.Read[uint64(readHost)].Get(c, key).Result()
		if err != nil {
			str, err = clients.Host.Get(c, key).Result()
		}
	} else {
		// If there is no read replica then just hit the host server.
		str, err = clients.Host.Get(c, key).Result()
	}

	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", errors.WithStack(err)
	}

	return str, nil
}

func RedisMGet(c context.Context, clients *RedisDBConn, keys ...string) ([]interface{}, error) {
	var err error
	var result []interface{}

	noOfReadReplica := len(clients.Read)

	// Check the read replicas are available or not.
	if noOfReadReplica == 1 {
		result, err = clients.Read[0].MGet(c, keys...).Result()
		if err != nil {
			result, err = clients.Host.MGet(c, keys...).Result()
		}
	} else if noOfReadReplica > 1 {
		// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
		rand.Seed(time.Now().UnixNano())
		readHost := rand.Intn(noOfReadReplica)

		result, err = clients.Read[uint64(readHost)].MGet(c, keys...).Result()
		if err != nil {
			result, err = clients.Host.MGet(c, keys...).Result()
		}
	} else {
		// If there is no read replica then just hit the host server.
		result, err = clients.Host.MGet(c, keys...).Result()
	}

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return result, nil
}

// To get single redis hash key and it's field from redis.
func RedisHGet(c context.Context, clients *RedisDBConn, key string, field string) (string, error) {
	var err error
	var result string

	noOfReadReplica := len(clients.Read)
	// Check the read replicas are available or not.
	if noOfReadReplica == 1 {
		result, err = clients.Read[0].HGet(c, key, field).Result()
		if err != nil {
			result, err = clients.Host.HGet(c, key, field).Result()
		}
	} else if noOfReadReplica > 1 {
		// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
		rand.Seed(time.Now().UnixNano())
		readHost := rand.Intn(noOfReadReplica)

		result, err = clients.Read[uint64(readHost)].HGet(c, key, field).Result()
		if err != nil {
			result, err = clients.Host.HGet(c, key, field).Result()
		}
	} else {
		// If there is no read replica then just hit the host server.
		result, err = clients.Host.HGet(c, key, field).Result()
	}

	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", errors.WithStack(err)
	}

	return result, nil
}

// To get one field from multiple redis hashes in one call to redis.
// Input is a map of keys and the respective field for those keys.
// Output is a map of keys and the respective values for those keys in redis.
func RedisHgets(c context.Context, clients *RedisDBConn, redisKeysWithField map[string]string) (map[string]string, error) {
	// Check the read replicas are available or not.
	noOfReadReplica := len(clients.Read)

	var pipe redis.Pipeliner
	if noOfReadReplica == 1 {
		pipe = clients.Read[0].Pipeline()
	} else if noOfReadReplica > 1 {
		// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
		rand.Seed(time.Now().UnixNano())
		readHost := rand.Intn(noOfReadReplica)
		pipe = clients.Read[uint64(readHost)].Pipeline()
	} else {
		// If there is no read replica then just hit the host server.
		pipe = clients.Host.Pipeline()
	}

	commandMapper := map[string]*redis.StringCmd{}
	for key, field := range redisKeysWithField {
		commandMapper[key] = pipe.HGet(c, key, field)
	}
	_, err := pipe.Exec(c)
	// for a key in the pipline for which the hget operation is being done
	// does not exist or the corresponding field for that key
	// does not exist redis marks it as redis.Nil error
	if err != nil && err != redis.Nil {
		return nil, errors.WithStack(err)
	}

	var mappedKeyFieldValues = make(map[string]string)
	// iterate through the commands and their responses from the pipeline execution.
	for _, v := range commandMapper {
		args := v.Args()
		redisKey := args[1].(string)
		mappedKeyFieldValues[redisKey] = v.Val()
	}
	return mappedKeyFieldValues, nil
}

func RedisGetLRange(c context.Context, clients *RedisDBConn, key string, start, stop int64) ([]string, error) {
	var err error
	var str []string

	noOfReadReplica := len(clients.Read)

	// Check the read replicas are available or not.
	if noOfReadReplica == 1 {
		str, err = clients.Read[0].LRange(c, key, start, stop).Result()
		if err != nil {
			str, err = clients.Host.LRange(c, key, start, stop).Result()
		}
	} else if noOfReadReplica > 1 {
		// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
		rand.Seed(time.Now().UnixNano())
		readHost := rand.Intn(noOfReadReplica)

		str, err = clients.Read[uint64(readHost)].LRange(c, key, start, stop).Result()
		if err != nil {
			str, err = clients.Host.LRange(c, key, start, stop).Result()
		}
	} else {
		// If there is no read replica then just hit the host server.
		str, err = clients.Host.LRange(c, key, start, stop).Result()
	}

	if err == redis.Nil {
		return []string{}, nil
	} else if err != nil {
		return []string{}, errors.WithStack(err)
	}

	return str, nil
}

func RedisSMembers(c context.Context, clients *RedisDBConn, key string) ([]string, error) {
	var err error
	var str []string

	noOfReadReplica := len(clients.Read)

	// Check the read replicas are available or not.
	if noOfReadReplica == 1 {
		str, err = clients.Read[0].SMembers(c, key).Result()
		if err != nil {
			str, err = clients.Host.SMembers(c, key).Result()
		}
	} else if noOfReadReplica > 1 {
		// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
		rand.Seed(time.Now().UnixNano())
		readHost := rand.Intn(noOfReadReplica)

		str, err = clients.Read[uint64(readHost)].SMembers(c, key).Result()
		if err != nil {
			str, err = clients.Host.SMembers(c, key).Result()
		}
	} else {
		// If there is no read replica then just hit the host server.
		str, err = clients.Host.SMembers(c, key).Result()
	}

	if err == redis.Nil {
		return []string{}, nil
	} else if err != nil {
		return []string{}, errors.WithStack(err)
	}

	return str, nil
}

func RedisSIsMember(c context.Context, clients *RedisDBConn, key string, element interface{}) (bool, error) {
	var err error
	var found bool

	noOfReadReplica := len(clients.Read)

	// Check the read replicas are available or not.
	if noOfReadReplica == 1 {
		found, err = clients.Read[0].SIsMember(c, key, element).Result()
		if err != nil {
			found, err = clients.Host.SIsMember(c, key, element).Result()
		}
	} else if noOfReadReplica > 1 {
		// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
		rand.Seed(time.Now().UnixNano())
		readHost := rand.Intn(noOfReadReplica)

		found, err = clients.Read[uint64(readHost)].SIsMember(c, key, element).Result()
		if err != nil {
			found, err = clients.Host.SIsMember(c, key, element).Result()
		}
	} else {
		// If there is no read replica then just hit the host server.
		found, err = clients.Host.SIsMember(c, key, element).Result()
	}

	if err != nil {
		return false, errors.WithStack(err)
	}

	return found, nil
}

func RedisSetJSON(c context.Context, clients *RedisDBConn, key string, data interface{}, ttl time.Duration) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := clients.Host.Set(c, key, string(jsonBytes), ttl).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisSet(c context.Context, clients *RedisDBConn, key string, data interface{}, ttl time.Duration) error {
	if err := clients.Host.Set(c, key, data, ttl).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisHSet(c context.Context, clients *RedisDBConn, key string, field string, data interface{}, ttl time.Duration) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := clients.Host.HSet(c, key, field, jsonBytes).Err(); err != nil {
		return errors.WithStack(err)
	}

	if ttl > 0 {
		if err := clients.Host.Expire(c, key, ttl).Err(); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func RedisRPush(c context.Context, clients *RedisDBConn, key string, valueList []string) error {
	if err := clients.Host.RPush(c, key, &valueList).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisIncrementValue(c context.Context, clients *RedisDBConn, key string) error {
	if err := clients.Host.Incr(c, key).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisSAdd(c context.Context, clients *RedisDBConn, key string, elements interface{}) error {
	if err := clients.Host.SAdd(c, key, elements).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisSRem(c context.Context, clients *RedisDBConn, key string, elements interface{}) error {
	if err := clients.Host.SRem(c, key, elements).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisDelKey(c context.Context, clients *RedisDBConn, keys ...string) error {
	if err := clients.Host.Del(c, keys...).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisExpireKey(c context.Context, clients *RedisDBConn, key string, ttl time.Duration) error {
	if err := clients.Host.Expire(c, key, ttl).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// getAllRedisTenantDB returns a singleton tenant db connection for each tenant.
func getAllRedisTenantDB(config RedisConfig, tenantCfgs []*TenantConnections, tenantAlterDbHostParam, tenantDBPassPhraseKey string) map[uint64]*RedisDBConn {

	tenantRedisDB := make(map[uint64]*RedisDBConn, len(tenantCfgs))

	for _, t := range tenantCfgs {

		var cfgsMap map[string]map[string]interface{}
		var err error
		if t.Connections != nil {
			if err = json.Unmarshal(t.Connections, &cfgsMap); err != nil {
				panic(err)
			}
		}

		// IMPORTANT: Check the `redis` object exist in the Connections column or not.
		if redisCfg, ok := cfgsMap["redis"]; ok {
			password := redisCfg["password"].(string)

			// IMPORTANT: If tenant database password is encrypted in master db config.
			if tenantDBPassPhraseKey != "" {
				password, err = aes.BeanAESDecrypt(tenantDBPassPhraseKey, password)
				if err != nil {
					panic(err)
				}
			}

			host := redisCfg["host"].(string)

			// IMPORTANT - If a command or service wants to use a different `host` parameter for tenant database connection
			// then it's easy to do just by passing that parameter string name using `bean.TenantAlterDbHostParam`.
			// Therfore, `bean` will overwrite all host string in `TenantConnections`.`Connections` JSON.
			if tenantAlterDbHostParam != "" && redisCfg[tenantAlterDbHostParam] != nil {
				host = redisCfg[tenantAlterDbHostParam].(string)
			}

			port := redisCfg["port"].(string)
			var dbName int
			if dbName, ok = redisCfg["database"].(int); !ok {
				dbName = 0
			}

			tenantRedisDB[t.TenantID] = &RedisDBConn{}

			tenantRedisDB[t.TenantID].Host, tenantRedisDB[t.TenantID].Name = connectRedisDB(
				password, host, port, dbName, config.Maxretries, config.PoolSize, config.MinIdleConnections,
				config.DialTimeout, config.ReadTimeout, config.WriteTimeout, config.PoolTimeout,
			)

			// IMPORTANT: Let's initialize the read replica connection if it is available.
			if readHostArray, ok := redisCfg["read"]; ok {
				if readHost, ok := readHostArray.([]interface{}); ok {
					redisReadConn := make(map[uint64]*redis.Client, len(readHost))

					for i, h := range readHost {
						var host, port string

						s := strings.Split(h.(string), ":")
						host = s[0]
						if len(s) != 2 {
							port = redisCfg["port"].(string)
						} else {
							port = s[1]
						}

						var dbName int
						if dbName, ok = redisCfg["database"].(int); !ok {
							dbName = 0
						}

						redisReadConn[uint64(i)], _ = connectRedisDB(
							password, host, port, dbName, config.Maxretries, config.PoolSize, config.MinIdleConnections,
							config.DialTimeout, config.ReadTimeout, config.WriteTimeout, config.PoolTimeout,
						)
					}

					tenantRedisDB[t.TenantID].Read = redisReadConn
				}
			}
		}
	}

	return tenantRedisDB
}

func connectRedisDB(
	password, host, port string, dbName int, maxretries, poolsize, minIdleConnections int,
	dialTimeout, readTimeout, writeTimeout, poolTimeout time.Duration,
) (*redis.Client, int) {

	rdb := redis.NewClient(&redis.Options{
		Addr:         host + ":" + port,
		Password:     password,
		DB:           dbName,
		MaxRetries:   maxretries,
		PoolSize:     poolsize,
		MinIdleConns: minIdleConnections,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		PoolTimeout:  poolTimeout,
	})

	return rdb, dbName
}

func GetRedisCachePrefix() string {
	return cachePrefix
}
