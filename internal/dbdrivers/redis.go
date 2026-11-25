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
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	"errors"

	"github.com/redis/go-redis/v9"
	"github.com/retail-ai-inc/bean/v3/aes"
	"github.com/retail-ai-inc/bean/v3/stacktrace"
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
		Host     string
		Port     string
		Reads    []string
		SSL      SSLConfig
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

func InitRedisTenantConns(config RedisConfig, masterMySQL *gorm.DB, tenantAlterDbHostParam, tenantDBPassPhraseKey string,
) (map[uint64]*RedisDBConn, []func() error, error) {
	cachePrefix = config.Prefix
	tenantCfgs, err := GetAllTenantCfgs(masterMySQL)
	if err != nil {
		return nil, noClosers, err
	}

	if len(tenantCfgs) > 0 {
		return getAllRedisTenantDB(config, tenantCfgs, tenantAlterDbHostParam, tenantDBPassPhraseKey)
	}

	return nil, noClosers, nil
}

func InitRedisMasterConn(config RedisConfig) (*RedisDBConn, []func() error, error) {

	var masterRedisDB *RedisDBConn
	var closeDBs []func() error

	masterCfg := config.Master
	if masterCfg != nil {

		var (
			close func() error
			err   error
		)

		masterRedisDB = &RedisDBConn{}
		masterRedisDB.Primary, masterRedisDB.Name, close, err = connectRedisDB(
			masterCfg.Password, masterCfg.Host, masterCfg.Port, masterCfg.Database,
			config.Maxretries, config.PoolSize, config.MinIdleConnections, config.DialTimeout,
			config.ReadTimeout, config.WriteTimeout, config.PoolTimeout, false, masterCfg.SSL,
		)
		if err != nil {
			return nil, noClosers, err
		}
		closeDBs = append(closeDBs, close)

		// when `len(strings.Split(masterCfg.Host, ","))>1`, it means that Redis will operate in `cluster` mode, and the `read` config will be ignored.
		if len(strings.Split(masterCfg.Host, ",")) > 1 {

			masterRedisDB.isCluster = true

		} else if len(strings.Split(masterCfg.Host, ",")) == 1 && len(masterCfg.Reads) > 0 {
			redisReadConns := make(map[uint64]redis.UniversalClient, len(masterCfg.Reads))

			for i, readHost := range masterCfg.Reads {
				redisReadConns[uint64(i)], _, close, err = connectRedisDB(
					masterCfg.Password, readHost, masterCfg.Port, masterCfg.Database,
					config.Maxretries, config.PoolSize, config.MinIdleConnections, config.DialTimeout,
					config.ReadTimeout, config.WriteTimeout, config.PoolTimeout, true, masterCfg.SSL,
				)
				if err != nil {
					return nil, noClosers, errors.Join(err, closeAll(closeDBs))
				}
				closeDBs = append(closeDBs, close)
			}

			masterRedisDB.Reads = redisReadConns
			masterRedisDB.readCount = len(masterRedisDB.Reads)
		}
	}

	return masterRedisDB, closeDBs, nil
}

func (clients *RedisDBConn) KeyExists(c context.Context, key string) (bool, error) {
	result, err := clients.Primary.Exists(c, key).Result()
	if err != nil {
		return false, stacktrace.WithStack(err)
	}

	if result == 1 {
		// if the key exists in redis.
		return true, nil
	}

	// if the key does not exist in redis.
	return false, nil
}

// redisReplicaDown records until when a read replica is skipped after a connection error (map[redis.UniversalClient]
// time.Time): the reads go to the primary meanwhile instead of waiting for the dial timeout of a replica that is down.
var redisReplicaDown sync.Map

// readWithFallback runs read on a random read replica, or on the primary in cluster mode or without replicas. A
// read failing on the replica because of the connection (not a miss, a server reply such as WRONGTYPE, or a canceled
// ctx) is retried on the primary, and the replica is skipped for `replicaRetryInterval`.
func readWithFallback[T any](c context.Context, clients *RedisDBConn, read func(redis.UniversalClient) (T, error)) (T, error) {
	if clients.isCluster || clients.readCount == 0 {
		return read(clients.Primary)
	}

	replica := clients.Reads[uint64(rand.Intn(clients.readCount))]
	if until, ok := redisReplicaDown.Load(replica); ok && time.Now().Before(until.(time.Time)) {
		return read(clients.Primary)
	}

	v, err := read(replica)
	if !shouldReadFromPrimary(c, err) {
		return v, err
	}
	redisReplicaDown.Store(replica, time.Now().Add(replicaRetryInterval))
	return read(clients.Primary)
}

// shouldReadFromPrimary reports whether a read that failed on a replica with err should be retried on the primary.
func shouldReadFromPrimary(c context.Context, err error) bool {
	if err == nil || errors.Is(err, redis.Nil) || c.Err() != nil {
		return false
	}
	var redisErr redis.Error
	if errors.As(err, &redisErr) {
		// A reply of the server: the primary would answer the same (e.g. WRONGTYPE), unless the replica is not
		// ready to serve reads.
		msg := redisErr.Error()
		return strings.HasPrefix(msg, "LOADING") || strings.HasPrefix(msg, "MASTERDOWN")
	}
	return true // connection error, timeout...
}

func (clients *RedisDBConn) TTL(c context.Context, key string) (time.Duration, error) {
	ttl, err := readWithFallback(c, clients, func(rc redis.UniversalClient) (time.Duration, error) {
		return rc.TTL(c, key).Result()
	})
	if err != nil && !errors.Is(err, redis.Nil) {
		return ttl, stacktrace.WithStack(err)
	}
	return ttl, nil
}

func (clients *RedisDBConn) Keys(c context.Context, pattern string) ([]string, error) {
	var keys []string
	var err error
	if clients.isCluster {
		// KEYS is not routed by key in a cluster (go-redis sends it to a random node): run it on every master.
		keys, err = clusterKeys(c, clients.Primary, pattern)
	} else {
		keys, err = readWithFallback(c, clients, func(rc redis.UniversalClient) ([]string, error) {
			return rc.Keys(c, pattern).Result()
		})
	}
	if err != nil && !errors.Is(err, redis.Nil) {
		return keys, stacktrace.WithStack(err)
	}
	return keys, nil
}

func (clients *RedisDBConn) GetString(c context.Context, key string) (string, error) {
	str, err := readWithFallback(c, clients, func(rc redis.UniversalClient) (string, error) {
		return rc.Get(c, key).Result()
	})
	if errors.Is(err, redis.Nil) {
		return "", nil
	} else if err != nil {
		return "", stacktrace.WithStack(err)
	}
	return str, nil
}

// MGet This is a replacement of the original `MGet` method by utilizing the `pipeline` approach when Redis is in `cluster` mode.
func (clients *RedisDBConn) MGet(c context.Context, keys ...string) ([]interface{}, error) {
	var result []interface{}
	var err error
	if clients.isCluster {
		result, err = wrapMGet(c, clients.Primary, keys...)
	} else {
		result, err = readWithFallback(c, clients, func(rc redis.UniversalClient) ([]interface{}, error) {
			return rc.MGet(c, keys...).Result()
		})
	}
	if err != nil {
		return nil, stacktrace.WithStack(err)
	}
	return result, nil
}

func (clients *RedisDBConn) HGet(c context.Context, key string, field string) (string, error) {
	result, err := readWithFallback(c, clients, func(rc redis.UniversalClient) (string, error) {
		return rc.HGet(c, key, field).Result()
	})
	if errors.Is(err, redis.Nil) {
		return "", nil
	} else if err != nil {
		return "", stacktrace.WithStack(err)
	}
	return result, nil
}

func (clients *RedisDBConn) HMGet(c context.Context, key string, fields ...string) ([]interface{}, error) {
	result, err := readWithFallback(c, clients, func(rc redis.UniversalClient) ([]interface{}, error) {
		return rc.HMGet(c, key, fields...).Result()
	})
	if errors.Is(err, redis.Nil) {
		return nil, nil
	} else if err != nil {
		return nil, stacktrace.WithStack(err)
	}
	return result, nil
}

func (clients *RedisDBConn) HGetAll(c context.Context, key string) (map[string]string, error) {
	result, err := readWithFallback(c, clients, func(rc redis.UniversalClient) (map[string]string, error) {
		return rc.HGetAll(c, key).Result()
	})
	if errors.Is(err, redis.Nil) {
		return nil, nil
	} else if err != nil {
		return nil, stacktrace.WithStack(err)
	}
	return result, nil
}

// HGets returns the value of the field of each key (map[key]field), "" for a missing one, using a pipeline.
func (clients *RedisDBConn) HGets(c context.Context, redisKeysWithField map[string]string) (map[string]string, error) {
	result, err := readWithFallback(c, clients, func(rc redis.UniversalClient) (map[string]string, error) {
		pipe := rc.Pipeline()
		cmds := make(map[string]*redis.StringCmd, len(redisKeysWithField))
		for key, field := range redisKeysWithField {
			cmds[key] = pipe.HGet(c, key, field)
		}
		_, _ = pipe.Exec(c) // the error of each command is checked below

		values := make(map[string]string, len(cmds))
		for key, cmd := range cmds {
			val, err := cmd.Result()
			if err != nil && !errors.Is(err, redis.Nil) {
				// Exec only returns the first error: a miss (redis.Nil) must not hide a real error.
				return nil, err
			}
			values[key] = val
		}
		return values, nil
	})
	if err != nil {
		return nil, stacktrace.WithStack(err)
	}
	return result, nil
}

func (clients *RedisDBConn) GetLRange(c context.Context, key string, start, stop int64) ([]string, error) {
	str, err := readWithFallback(c, clients, func(rc redis.UniversalClient) ([]string, error) {
		return rc.LRange(c, key, start, stop).Result()
	})
	if errors.Is(err, redis.Nil) {
		return []string{}, nil
	} else if err != nil {
		return []string{}, stacktrace.WithStack(err)
	}
	return str, nil
}

func (clients *RedisDBConn) SMembers(c context.Context, key string) ([]string, error) {
	str, err := readWithFallback(c, clients, func(rc redis.UniversalClient) ([]string, error) {
		return rc.SMembers(c, key).Result()
	})
	if errors.Is(err, redis.Nil) {
		return []string{}, nil
	} else if err != nil {
		return []string{}, stacktrace.WithStack(err)
	}
	return str, nil
}

func (clients *RedisDBConn) SIsMember(c context.Context, key string, element interface{}) (bool, error) {
	found, err := readWithFallback(c, clients, func(rc redis.UniversalClient) (bool, error) {
		return rc.SIsMember(c, key, element).Result()
	})
	if err != nil {
		return false, stacktrace.WithStack(err)
	}
	return found, nil
}

func (clients *RedisDBConn) SRandMemberN(c context.Context, key string, count int64) ([]string, error) {
	result, err := readWithFallback(c, clients, func(rc redis.UniversalClient) ([]string, error) {
		return rc.SRandMemberN(c, key, count).Result()
	})
	if err != nil {
		return nil, stacktrace.WithStack(err)
	}
	return result, nil
}

func (clients *RedisDBConn) SetJSON(c context.Context, key string, data interface{}, ttl time.Duration) error {
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return stacktrace.WithStack(err)
	}

	if err := clients.Primary.Set(c, key, string(jsonBytes), ttl).Err(); err != nil {
		return stacktrace.WithStack(err)
	}

	return nil
}

func (clients *RedisDBConn) Set(c context.Context, key string, data interface{}, ttl time.Duration) error {
	if err := clients.Primary.Set(c, key, data, ttl).Err(); err != nil {
		return stacktrace.WithStack(err)
	}

	return nil
}

// SetNX sets key to data only if the key does not already exist.
// Returns true when the key was set, false when it already existed.
func (clients *RedisDBConn) SetNX(c context.Context, key string, data interface{}, ttl time.Duration) (bool, error) {
	ok, err := clients.Primary.SetNX(c, key, data, ttl).Result()
	if err != nil {
		return false, stacktrace.WithStack(err)
	}

	return ok, nil
}

// - HSet("myhash", map[string]interface{}{"key1": "value1", "key2": "value2"})
// - HSet("myhash", []string{"key1", "value1", "key2", "value2"})
// - HSet("myhash", "key1", "value1", "key2", "value2")
func (clients *RedisDBConn) HSet(c context.Context, key string, args ...interface{}) error {
	if len(args) == 0 {
		return stacktrace.WithStack(ErrRedisInvalidParameter)
	}
	if err := clients.Primary.HSet(c, key, args...).Err(); err != nil {
		return stacktrace.WithStack(err)
	}
	return nil
}

func (clients *RedisDBConn) RPush(c context.Context, key string, valueList []string) error {
	if len(valueList) == 0 {
		return nil
	}
	// go-redis expands a single []string argument into the values.
	if err := clients.Primary.RPush(c, key, valueList).Err(); err != nil {
		return stacktrace.WithStack(err)
	}

	return nil
}

func (clients *RedisDBConn) IncrementValue(c context.Context, key string) error {
	if err := clients.Primary.Incr(c, key).Err(); err != nil {
		return stacktrace.WithStack(err)
	}

	return nil
}

func (clients *RedisDBConn) SAdd(c context.Context, key string, elements interface{}) error {
	if err := clients.Primary.SAdd(c, key, elements).Err(); err != nil {
		return stacktrace.WithStack(err)
	}

	return nil
}

func (clients *RedisDBConn) SRem(c context.Context, key string, elements interface{}) error {
	if err := clients.Primary.SRem(c, key, elements).Err(); err != nil {
		return stacktrace.WithStack(err)
	}

	return nil
}

func (clients *RedisDBConn) DelKey(c context.Context, keys ...string) error {
	if clients.isCluster && len(keys) > 1 {
		// A multi-key DEL fails with CROSSSLOT when the keys are in different slots: delete them one by one in a
		// pipeline (go-redis routes each command to the node of its key).
		_, err := clients.Primary.Pipelined(c, func(pipe redis.Pipeliner) error {
			for _, key := range keys {
				pipe.Del(c, key)
			}
			return nil
		})
		if err != nil {
			return stacktrace.WithStack(err)
		}
		return nil
	}

	if err := clients.Primary.Del(c, keys...).Err(); err != nil {
		return stacktrace.WithStack(err)
	}

	return nil
}

func (clients *RedisDBConn) ExpireKey(c context.Context, key string, ttl time.Duration) error {
	if err := clients.Primary.Expire(c, key, ttl).Err(); err != nil {
		return stacktrace.WithStack(err)
	}

	return nil
}
func (clients *RedisDBConn) Pipeline() redis.Pipeliner {
	return clients.Primary.Pipeline()
}

func (clients *RedisDBConn) Pipelined(c context.Context, fn func(redis.Pipeliner) error) ([]redis.Cmder, error) {
	return clients.Primary.Pipelined(c, fn)
}

// MSet This is a replacement of the original `MSet` method by utilizing the `pipeline` approach when Redis is in `cluster` mode.
// it accepts multiple values:
//   - RedisMSet("key1", "value1", "key2", "value2")
//   - RedisMSet([]string{"key1", "value1", "key2", "value2"})
//   - RedisMSet(map[string]interface{}{"key1": "value1", "key2": "value2"})
//
// For `struct` values, please implement the `encoding.BinaryMarshaler` interface.
func (clients *RedisDBConn) MSet(c context.Context, values ...interface{}) (err error) {
	if clients.isCluster {
		err = wrapMSet(c, clients.Primary, 0, values...)
	} else {
		_, err = clients.Primary.MSet(c, values...).Result()
	}

	if err != nil {
		return stacktrace.WithStack(err)
	}

	return nil
}

// MSetWithTTL
// This method is implemented using `pipeline`.
// For accepts multiple values, see RedisMSet description.
func (clients *RedisDBConn) MSetWithTTL(c context.Context, ttl time.Duration, values ...interface{}) (err error) {
	if err = wrapMSet(c, clients.Primary, ttl, values...); err != nil {
		return stacktrace.WithStack(err)
	}

	return nil
}

// Eval will always be executed on the primary redis server.
func (clients *RedisDBConn) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	v, err := clients.Primary.Eval(ctx, script, keys, args...).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, stacktrace.WithStack(err)
	}
	return v, nil
}

func (clients *RedisDBConn) EvalSha(ctx context.Context, sha1 string, keys []string, args ...interface{}) (interface{}, error) {
	v, err := clients.Primary.EvalSha(ctx, sha1, keys, args...).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, stacktrace.WithStack(err)
	}
	return v, nil
}

// Run wraps redis.Script.
func (clients *RedisDBConn) Run(ctx context.Context, script *redis.Script, keys []string, args ...interface{}) (interface{}, error) {
	v, err := script.Run(ctx, clients.Primary, keys, args...).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, nil
		}
		return nil, stacktrace.WithStack(err)
	}
	return v, nil
}

func wrapMSet(ctx context.Context, clients redis.UniversalClient, ttl time.Duration, values ...interface{}) error {
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
	for i := 0; i < len(dst); i += 2 {
		if _, ok := dst[i].(string); !ok {
			return fmt.Errorf("%w: key %v is not a string", ErrRedisInvalidParameter, dst[i])
		}
	}
	_, err := clients.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for i := 0; i < len(dst); i += 2 {
			pipe.Set(ctx, dst[i].(string), dst[i+1], ttl)
		}
		return nil
	})
	return err
}

// wrapMGet is MGET for a cluster: one GET per key in a pipeline (go-redis routes each command to the node of its
// key). Like MGET, the result has one entry per key: the string value, or nil if the key does not exist.
func wrapMGet(ctx context.Context, clients redis.UniversalClient, keys ...string) ([]interface{}, error) {
	cmds := make([]*redis.StringCmd, len(keys))
	_, err := clients.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for i, key := range keys {
			cmds[i] = pipe.Get(ctx, key)
		}
		return nil
	})
	// Pipelined returns the first command error, which is redis.Nil for a missing key: check each command instead.
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, err
	}

	results := make([]interface{}, len(keys))
	for i, cmd := range cmds {
		val, err := cmd.Result()
		switch {
		case errors.Is(err, redis.Nil):
			results[i] = nil
		case err != nil:
			return nil, err
		default:
			results[i] = val
		}
	}
	return results, nil
}

// clusterKeys runs KEYS on every master of a cluster and returns all the matching keys.
func clusterKeys(ctx context.Context, client redis.UniversalClient, pattern string) ([]string, error) {
	cluster, ok := client.(*redis.ClusterClient)
	if !ok {
		return client.Keys(ctx, pattern).Result()
	}

	var (
		mu   sync.Mutex
		keys []string
	)
	err := cluster.ForEachMaster(ctx, func(ctx context.Context, master *redis.Client) error {
		masterKeys, err := master.Keys(ctx, pattern).Result()
		if err != nil {
			return err
		}
		mu.Lock()
		keys = append(keys, masterKeys...)
		mu.Unlock()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return keys, nil
}

// getAllRedisTenantDB returns a singleton tenant db connection for each tenant. On error, the connections already
// opened are closed.
func getAllRedisTenantDB(config RedisConfig, tenantCfgs []*TenantConnections, tenantAlterDbHostParam, tenantDBPassPhraseKey string,
) (map[uint64]*RedisDBConn, []func() error, error) {

	tenantRedisDB := make(map[uint64]*RedisDBConn, len(tenantCfgs))
	closers := make([]func() error, 0, len(tenantCfgs))

	fail := func(t *TenantConnections, msg string, err error) (map[uint64]*RedisDBConn, []func() error, error) {
		return nil, noClosers, errors.Join(
			fmt.Errorf("%s (%d:%s): %w", msg, t.TenantID, t.Code, err), closeAll(closers))
	}

	for _, t := range tenantCfgs {

		var cfgsMap map[string]map[string]interface{}
		if t.Connections != nil {
			if err := json.Unmarshal(t.Connections, &cfgsMap); err != nil {
				return fail(t, "failed to unmarshal tenant connections", err)
			}
		}

		// IMPORTANT: Check the `redis` object exist in the Connections column or not.
		redisCfg, ok := cfgsMap["redis"]
		if !ok {
			continue
		}

		password, err := requiredTenantString(redisCfg, "password")
		if err != nil {
			return fail(t, "invalid redis tenant config", err)
		}

		// IMPORTANT: If tenant database password is encrypted in master mysql db config.
		if tenantDBPassPhraseKey != "" {
			password, err = aes.BeanAESDecrypt(tenantDBPassPhraseKey, password)
			if err != nil {
				return fail(t, "failed to decrypt redis tenant database password", err)
			}
		}

		// IMPORTANT - If a command or service wants to use a different `host` parameter for tenant database connection
		// then it's easy to do just by passing that parameter string name using `bean.TenantAlterDbHostParam`.
		// Therfore, `bean` will overwrite all host string in `TenantConnections`.`Connections` JSON.
		host, err := tenantHost(redisCfg, tenantAlterDbHostParam)
		if err != nil {
			return fail(t, "invalid redis tenant config", err)
		}

		port, err := requiredTenantString(redisCfg, "port")
		if err != nil {
			return fail(t, "invalid redis tenant config", err)
		}
		var dbName int
		if _dbName, ok := redisCfg["database"].(float64); ok {
			dbName = int(_dbName)
		}
		ssl := sslConfigFromMap(redisCfg)

		conn := &RedisDBConn{}
		var closeDB func() error
		conn.Primary, conn.Name, closeDB, err = connectRedisDB(
			password, host, port, dbName, config.Maxretries, config.PoolSize, config.MinIdleConnections,
			config.DialTimeout, config.ReadTimeout, config.WriteTimeout, config.PoolTimeout, false, ssl,
		)
		if err != nil {
			return fail(t, "failed to connect redis tenant database", err)
		}
		closers = append(closers, closeDB)
		tenantRedisDB[t.TenantID] = conn

		// IMPORTANT: Let's initialize the read replica connection if it is available.
		// when `len(strings.Split(host, ","))>1`, it means that Redis will operate in `cluster` mode, and the `read` config will be ignored.
		if len(strings.Split(host, ",")) > 1 {
			conn.isCluster = true
			continue
		}

		readHosts, err := readHostsFromMap(redisCfg)
		if err != nil {
			return fail(t, "invalid redis tenant read replicas", err)
		}
		if len(readHosts) == 0 {
			continue
		}

		redisReadConn := make(map[uint64]redis.UniversalClient, len(readHosts))
		for i, readHost := range readHosts {
			redisReadConn[uint64(i)], _, closeDB, err = connectRedisDB(
				password, readHost, port, dbName, config.Maxretries, config.PoolSize, config.MinIdleConnections,
				config.DialTimeout, config.ReadTimeout, config.WriteTimeout, config.PoolTimeout, true, ssl,
			)
			if err != nil {
				return fail(t, "failed to connect to redis tenant database read replica", err)
			}
			closers = append(closers, closeDB)
		}

		conn.Reads = redisReadConn
		conn.readCount = len(conn.Reads)
	}

	return tenantRedisDB, closers, nil
}

func connectRedisDB(
	password, host, port string, dbName int, maxretries, poolsize, minIdleConnections int,
	dialTimeout, readTimeout, writeTimeout, poolTimeout time.Duration, readOnly bool, ssl SSLConfig,
) (redis.UniversalClient, int, func() error, error) {

	hosts := strings.Split(host, ",")
	for i, h := range hosts {
		hs := strings.Split(h, ":")
		if len(hs) == 1 {
			hosts[i] = strings.Join([]string{h, port}, ":")
		}
	}

	tlsConfig, releaseTLS, err := newTLSConfig(ssl, hostsWithoutPort(host))
	if err != nil {
		return nil, 0, noClose, err
	}

	rdb := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        hosts,
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
		TLSConfig:    tlsConfig,
	})
	closeClient := func() error {
		defer releaseTLS()
		return rdb.Close()
	}

	// Check the connection
	_, err = rdb.Ping(context.TODO()).Result()
	if err != nil {
		return nil, 0, noClose, errors.Join(fmt.Errorf("redis connection error: %w", err), closeClient())
	}

	return rdb, dbName, closeClient, nil
}

func GetRedisCachePrefix() string {
	return cachePrefix
}
