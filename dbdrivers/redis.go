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

var ErrRedisInvalidParameter = errors.New("redis invalid parameter")

// RedisDBConn IMPORTANT: This structure is holding any kind of redis connection using a map in bean.go.
type RedisDBConn struct {
	Primary   redis.UniversalClient
	Reads     map[uint64]redis.UniversalClient
	Name      int
	readCount int
	isCluster bool
}

type RedisConfig struct {
	Master *struct {
		Database int
		Password string
		Hosts    string // Hosts can be multiple hosts separated by comma.
		Port     string
		Reads    []string
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

func InitRedisMasterConn(config RedisConfig) *RedisDBConn {

	var masterRedisDB *RedisDBConn

	masterCfg := config.Master
	if masterCfg != nil {

		masterRedisDB = &RedisDBConn{}

		masterRedisDB.Primary, masterRedisDB.Name = connectRedisDB(
			masterCfg.Password, masterCfg.Hosts, masterCfg.Port, masterCfg.Database,
			config.Maxretries, config.PoolSize, config.MinIdleConnections, config.DialTimeout,
			config.ReadTimeout, config.WriteTimeout, config.PoolTimeout, false,
		)

		// when `len(strings.Split(masterCfg.Hosts, ","))>1`, it means that Redis will operate in `cluster` mode, and the `read` config will be ignored.
		if len(strings.Split(masterCfg.Hosts, ",")) > 1 {

			masterRedisDB.isCluster = true

		} else if len(strings.Split(masterCfg.Hosts, ",")) == 1 && len(masterCfg.Reads) > 0 {
			redisReadconns := make(map[uint64]redis.UniversalClient, len(masterCfg.Reads))

			for i, readHost := range masterCfg.Reads {
				redisReadconns[uint64(i)], _ = connectRedisDB(
					masterCfg.Password, readHost, masterCfg.Port, masterCfg.Database,
					config.Maxretries, config.PoolSize, config.MinIdleConnections, config.DialTimeout,
					config.ReadTimeout, config.WriteTimeout, config.PoolTimeout, true,
				)
			}

			masterRedisDB.Reads = redisReadconns
			masterRedisDB.readCount = len(masterRedisDB.Reads)
		} else {
			// TODO: do something here in an edge case? panic?
		}
	}

	return masterRedisDB
}

func RedisIsKeyExists(c context.Context, conn *RedisDBConn, key string) (bool, error) {
	result, err := conn.Primary.Exists(c, key).Result()
	if err != nil {
		return false, errors.WithStack(err)
	}

	if result == 1 {
		// if the key exists in redis.
		return true, nil
	}

	// if the key does not exist in redis.
	return false, nil
}

func RedisGetString(c context.Context, conn *RedisDBConn, key string) (str string, err error) {

	if conn.isCluster {
		// If client is cluster mode then just hit the primary server.
		str, err = conn.Primary.Get(c, key).Result()
	} else {
		// Check the read replicas are available or not.
		if conn.readCount == 1 {
			str, err = conn.Reads[0].Get(c, key).Result()
			if err != nil {
				str, err = conn.Primary.Get(c, key).Result()
			}
		} else if conn.readCount > 1 {
			// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
			// TODO: Use global seed and make go version as 1.20 minimum.
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			readHost := rng.Intn(conn.readCount)

			str, err = conn.Reads[uint64(readHost)].Get(c, key).Result()
			if err != nil {
				str, err = conn.Primary.Get(c, key).Result()
			}
		} else {
			// If there is no read replica then just hit the primary server.
			str, err = conn.Primary.Get(c, key).Result()
		}
	}

	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", errors.WithStack(err)
	}

	return str, nil
}

// RedisMGet This is a replacement of the original `MGet` method by utilizing the `pipeline` approach when Redis is in `cluster` mode.
func RedisMGet(c context.Context, conn *RedisDBConn, keys ...string) (result []interface{}, err error) {

	if conn.isCluster {
		// If client is cluster mode then just hit the primary server.
		result, err = conn.wrapMGet(c, keys...)
	} else {
		// Check the read replicas are available or not.
		if conn.readCount == 1 {
			result, err = conn.Reads[0].MGet(c, keys...).Result()
			if err != nil {
				result, err = conn.Primary.MGet(c, keys...).Result()
			}
		} else if conn.readCount > 1 {
			// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
			// TODO: Use global seed and make go version as 1.20 minimum.
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			readHost := rng.Intn(conn.readCount)

			result, err = conn.Reads[uint64(readHost)].MGet(c, keys...).Result()
			if err != nil {
				result, err = conn.Primary.MGet(c, keys...).Result()
			}
		} else {
			// If there is no read replica then just hit the primary server.
			result, err = conn.Primary.MGet(c, keys...).Result()
		}
	}

	if err != nil {
		return nil, errors.WithStack(err)
	}

	return result, nil
}

// RedisHGet To get single redis hash key and it's field from redis.
func RedisHGet(c context.Context, conn *RedisDBConn, key string, field string) (result string, err error) {

	if conn.isCluster {
		// If client is cluster mode then just hit the primary server.
		result, err = conn.Primary.HGet(c, key, field).Result()
	} else {
		// Check the read replicas are available or not.
		if conn.readCount == 1 {
			result, err = conn.Reads[0].HGet(c, key, field).Result()
			if err != nil {
				result, err = conn.Primary.HGet(c, key, field).Result()
			}
		} else if conn.readCount > 1 {
			// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
			// TODO: Use global seed and make go version as 1.20 minimum.
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			readHost := rng.Intn(conn.readCount)

			result, err = conn.Reads[uint64(readHost)].HGet(c, key, field).Result()
			if err != nil {
				result, err = conn.Primary.HGet(c, key, field).Result()
			}
		} else {
			// If there is no read replica then just hit the primary server.
			result, err = conn.Primary.HGet(c, key, field).Result()
		}
	}

	if err == redis.Nil {
		return "", nil
	} else if err != nil {
		return "", errors.WithStack(err)
	}

	return result, nil
}

// RedisHgets To get one field from multiple redis hashes in one call to redis.
// Input is a map of keys and the respective field for those keys.
// Output is a map of keys and the respective values for those keys in redis.
func RedisHgets(c context.Context, conn *RedisDBConn, redisKeysWithField map[string]string) (map[string]string, error) {

	var pipe redis.Pipeliner
	if conn.isCluster {
		// If client is cluster mode then just hit the primary server.
		pipe = conn.Primary.Pipeline()
	} else {
		// Check the read replicas are available or not.
		if conn.readCount == 1 {
			pipe = conn.Reads[0].Pipeline()
		} else if conn.readCount > 1 {
			// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
			// TODO: Use global seed and make go version as 1.20 minimum.
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			readHost := rng.Intn(conn.readCount)
			pipe = conn.Reads[uint64(readHost)].Pipeline()
		} else {
			// If there is no read replica then just hit the primary server.
			pipe = conn.Primary.Pipeline()
		}
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

func RedisGetLRange(c context.Context, conn *RedisDBConn, key string, start, stop int64) (str []string, err error) {

	if conn.isCluster {
		// If client is cluster mode then just hit the primary server.
		str, err = conn.Primary.LRange(c, key, start, stop).Result()
	} else {
		// Check the read replicas are available or not.
		if conn.readCount == 1 {
			str, err = conn.Reads[0].LRange(c, key, start, stop).Result()
			if err != nil {
				str, err = conn.Primary.LRange(c, key, start, stop).Result()
			}
		} else if conn.readCount > 1 {
			// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
			// TODO: Use global seed and make go version as 1.20 minimum.
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			readHost := rng.Intn(conn.readCount)

			str, err = conn.Reads[uint64(readHost)].LRange(c, key, start, stop).Result()
			if err != nil {
				str, err = conn.Primary.LRange(c, key, start, stop).Result()
			}
		} else {
			// If there is no read replica then just hit the primary server.
			str, err = conn.Primary.LRange(c, key, start, stop).Result()
		}
	}

	if err == redis.Nil {
		return []string{}, nil
	} else if err != nil {
		return []string{}, errors.WithStack(err)
	}

	return str, nil
}

func RedisSMembers(c context.Context, conn *RedisDBConn, key string) (str []string, err error) {

	if conn.isCluster {
		// If client is cluster mode then just hit the primary server.
		str, err = conn.Primary.SMembers(c, key).Result()
	} else {
		// Check the read replicas are available or not.
		if conn.readCount == 1 {
			str, err = conn.Reads[0].SMembers(c, key).Result()
			if err != nil {
				str, err = conn.Primary.SMembers(c, key).Result()
			}
		} else if conn.readCount > 1 {
			// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
			// TODO: Use global seed and make go version as 1.20 minimum.
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			readHost := rng.Intn(conn.readCount)

			str, err = conn.Reads[uint64(readHost)].SMembers(c, key).Result()
			if err != nil {
				str, err = conn.Primary.SMembers(c, key).Result()
			}
		} else {
			// If there is no read replica then just hit the primary server.
			str, err = conn.Primary.SMembers(c, key).Result()
		}
	}

	if err == redis.Nil {
		return []string{}, nil
	} else if err != nil {
		return []string{}, errors.WithStack(err)
	}

	return str, nil
}

func RedisSIsMember(c context.Context, conn *RedisDBConn, key string, element interface{}) (found bool, err error) {

	if conn.isCluster {
		// If client is cluster mode then just hit the primary server.
		found, err = conn.Primary.SIsMember(c, key, element).Result()
	} else {
		// Check the read replicas are available or not.
		if conn.readCount == 1 {
			found, err = conn.Reads[0].SIsMember(c, key, element).Result()
			if err != nil {
				found, err = conn.Primary.SIsMember(c, key, element).Result()
			}
		} else if conn.readCount > 1 {
			// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
			// TODO: Use global seed and make go version as 1.20 minimum.
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			readHost := rng.Intn(conn.readCount)

			found, err = conn.Reads[uint64(readHost)].SIsMember(c, key, element).Result()
			if err != nil {
				found, err = conn.Primary.SIsMember(c, key, element).Result()
			}
		} else {
			// If there is no read replica then just hit the primary server.
			found, err = conn.Primary.SIsMember(c, key, element).Result()
		}
	}

	if err != nil {
		return false, errors.WithStack(err)
	}

	return found, nil
}

func RedisSRandMemberN(c context.Context, conn *RedisDBConn, key string, count int64) (result []string, err error) {

	if conn.isCluster {
		// If client is cluster mode then just hit the primary server.
		result, err = conn.Primary.SRandMemberN(c, key, count).Result()
	} else {
		// Check the read replicas are available or not.
		if conn.readCount == 1 {
			result, err = conn.Reads[0].SRandMemberN(c, key, count).Result()
			if err != nil {
				return nil, errors.WithStack(err)
			}
		} else if conn.readCount > 1 {
			// Select a read replica between 0 ~ noOfReadReplica-1 randomly.
			// TODO: Use global seed and make go version as 1.20 minimum.
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			readHost := rng.Intn(conn.readCount)

			result, err = conn.Reads[uint64(readHost)].SRandMemberN(c, key, count).Result()
			if err != nil {
				return nil, errors.WithStack(err)
			}
		} else {
			// If there is no read replica then just hit the primary server.
			result, err = conn.Primary.SRandMemberN(c, key, count).Result()
		}
	}

	return result, err
}

func RedisSetJSON(c context.Context, conn *RedisDBConn, key string, data interface{}, ttl time.Duration) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := conn.Primary.Set(c, key, string(jsonBytes), ttl).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisSet(c context.Context, conn *RedisDBConn, key string, data interface{}, ttl time.Duration) error {
	if err := conn.Primary.Set(c, key, data, ttl).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisHSet(c context.Context, conn *RedisDBConn, key string, field string, data interface{}, ttl time.Duration) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return errors.WithStack(err)
	}

	if err := conn.Primary.HSet(c, key, field, jsonBytes).Err(); err != nil {
		return errors.WithStack(err)
	}

	if ttl > 0 {
		if err := conn.Primary.Expire(c, key, ttl).Err(); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

func RedisRPush(c context.Context, conn *RedisDBConn, key string, valueList []string) error {
	if err := conn.Primary.RPush(c, key, &valueList).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisIncrementValue(c context.Context, conn *RedisDBConn, key string) error {
	if err := conn.Primary.Incr(c, key).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisSAdd(c context.Context, conn *RedisDBConn, key string, elements interface{}) error {
	if err := conn.Primary.SAdd(c, key, elements).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisSRem(c context.Context, conn *RedisDBConn, key string, elements interface{}) error {
	if err := conn.Primary.SRem(c, key, elements).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisDelKey(c context.Context, conn *RedisDBConn, keys ...string) error {
	if err := conn.Primary.Del(c, keys...).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func RedisExpireKey(c context.Context, conn *RedisDBConn, key string, ttl time.Duration) error {
	if err := conn.Primary.Expire(c, key, ttl).Err(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// RedisMSet This is a replacement of the original `MSet` method by utilizing the `pipeline` approach when Redis is in `cluster` mode.
// it accepts multiple values:
//   - RedisMSet("key1", "value1", "key2", "value2")
//   - RedisMSet([]string{"key1", "value1", "key2", "value2"})
//   - RedisMSet(map[string]interface{}{"key1": "value1", "key2": "value2"})
//
// For `struct` values, please implement the `encoding.BinaryMarshaler` interface.
func RedisMSet(c context.Context, conn *RedisDBConn, values ...interface{}) (err error) {
	if conn.isCluster {
		err = conn.wrapMSet(c, 0, values...)
	} else {
		_, err = conn.Primary.MSet(c, values...).Result()
	}

	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// RedisMSetWithTTL
// This method is implemented using `pipeline`.
// For accepts multiple values, see RedisMSet description.
func RedisMSetWithTTL(c context.Context, conn *RedisDBConn, ttl time.Duration, values ...interface{}) (err error) {
	if err = conn.wrapMSet(c, ttl, values...); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (c *RedisDBConn) wrapMSet(ctx context.Context, ttl time.Duration, values ...interface{}) error {
	var dst []interface{}
	switch len(values) {
	case 0:
	case 1:
		arg := values[0]
		switch arg := arg.(type) {
		case []string:
			for _, s := range arg {
				dst = append(dst, s)
			}
		case []interface{}:
			dst = append(dst, arg...)
		case map[string]interface{}:
			for k, v := range arg {
				dst = append(dst, k, v)
			}
		case map[string]string:
			for k, v := range arg {
				dst = append(dst, k, v)
			}
		default:
			dst = append(dst, arg)
		}
	default:
		dst = append(dst, values...)
	}
	if len(dst) == 0 || len(dst)%2 != 0 {
		return ErrRedisInvalidParameter
	}
	_, err := c.Primary.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for i := 0; i < len(dst); i += 2 {
			cmd := pipe.Set(ctx, dst[i].(string), dst[i+1], ttl)
			if cmd.Err() != nil {
				return cmd.Err()
			}
		}
		return nil
	})
	return err
}

func (c *RedisDBConn) wrapMGet(ctx context.Context, keys ...string) ([]interface{}, error) {
	var results = make([]interface{}, 0, len(keys))
	cmder, err := c.Primary.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for i := 0; i < len(keys); i++ {
			_, err := pipe.Get(ctx, keys[i]).Result()
			if err != nil {
				return err
			}
		}
		return nil
	})
	if errors.Is(err, redis.Nil) {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	for _, cmdRes := range cmder {
		results = append(results, cmdRes.(*redis.StringCmd).Val())
	}
	return results, nil
}

// getAllRedisTenantDB returns a singleton tenant db connection for each tenant.
func getAllRedisTenantDB(config RedisConfig, tenantCfgs []*TenantConnections, tenantAlterDbHostParam, tenantDBPassPhraseKey string) map[uint64]*RedisDBConn {

	tenantRedisDBs := make(map[uint64]*RedisDBConn, len(tenantCfgs))

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

			tenantRedisDBs[t.TenantID] = &RedisDBConn{}

			tenantRedisDBs[t.TenantID].Primary, tenantRedisDBs[t.TenantID].Name = connectRedisDB(
				password, host, port, dbName, config.Maxretries, config.PoolSize, config.MinIdleConnections,
				config.DialTimeout, config.ReadTimeout, config.WriteTimeout, config.PoolTimeout, false,
			)

			// IMPORTANT: Let's initialize the read replica connection if it is available.
			// when `len(strings.Split(host, ","))>1`, it means that Redis will operate in `cluster` mode, and the `read` config will be ignored.
			if len(strings.Split(host, ",")) > 1 {

				tenantRedisDBs[t.TenantID].isCluster = true

			} else if readHostArray, ok := redisCfg["read"]; ok && len(strings.Split(host, ",")) == 1 {
				if readHost, ok := readHostArray.([]interface{}); ok {
					redisReadConn := make(map[uint64]redis.UniversalClient, len(readHost))

					for i, h := range readHost {

						var host, port = h.(string), redisCfg["port"].(string)

						redisReadConn[uint64(i)], _ = connectRedisDB(
							password, host, port, dbName, config.Maxretries, config.PoolSize, config.MinIdleConnections,
							config.DialTimeout, config.ReadTimeout, config.WriteTimeout, config.PoolTimeout, true,
						)
					}

					tenantRedisDBs[t.TenantID].Reads = redisReadConn
					tenantRedisDBs[t.TenantID].readCount = len(tenantRedisDBs[t.TenantID].Reads)
				}
			}
		}
	}

	return tenantRedisDBs
}

func connectRedisDB(
	password, hosts, port string, dbName int, maxretries, poolsize, minIdleConnections int,
	dialTimeout, readTimeout, writeTimeout, poolTimeout time.Duration, readOnly bool,
) (redis.UniversalClient, int) {

	addrs := strings.Split(hosts, ",")
	for i, a := range addrs {
		as := strings.Split(a, ":")
		if len(as) == 1 {
			addrs[i] = strings.Join([]string{a, port}, ":")
		}
	}

	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        addrs,
		Password:     password,
		DB:           dbName,
		MaxRetries:   maxretries,
		PoolSize:     poolsize,
		MinIdleConns: minIdleConnections,
		DialTimeout:  dialTimeout,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
		PoolTimeout:  poolTimeout,
		ReadOnly:     readOnly,
	})
	// Check the connection
	_, err := rdb.Ping(context.TODO()).Result()
	if err != nil {
		panic(err)
	}

	return rdb, dbName
}

func GetRedisCachePrefix() string {
	return cachePrefix
}
