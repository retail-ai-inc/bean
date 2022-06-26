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
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/retail-ai-inc/bean/aes"
	"gorm.io/gorm"
)

type RedisConfig struct {
	Master *struct {
		Database int
		Password string
		Host     string
		Port     string
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

var cachePrefix string

func InitRedisTenantConns(config RedisConfig, master *gorm.DB, tenantDBPassPhraseKey string) (map[uint64]*redis.Client, map[uint64]int) {
	cachePrefix = config.Prefix
	tenantCfgs := GetAllTenantCfgs(master)

	return getAllRedisTenantDB(config, tenantCfgs, tenantDBPassPhraseKey)
}

func InitRedisMasterConn(config RedisConfig) (*redis.Client, int) {

	masterCfg := config.Master

	if masterCfg != nil {
		return connectRedisDB(
			masterCfg.Password, masterCfg.Host, masterCfg.Port, masterCfg.Database,
			config.Maxretries, config.PoolSize, config.MinIdleConnections, config.DialTimeout,
			config.ReadTimeout, config.WriteTimeout, config.PoolTimeout,
		)
	}

	return nil, -1
}

// GetTenantDB returns a singleton tenant db connection.
func getAllRedisTenantDB(config RedisConfig, tenantCfgs []*TenantConnections, tenantDBPassPhraseKey string) (map[uint64]*redis.Client, map[uint64]int) {

	redisConns := make(map[uint64]*redis.Client, len(tenantCfgs))
	redisDBNames := make(map[uint64]int, len(tenantCfgs))

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
			port := redisCfg["port"].(string)
			var dbName int
			if dbName, ok = redisCfg["database"].(int); !ok {
				dbName = 0
			}

			redisConns[t.TenantID], redisDBNames[t.TenantID] = connectRedisDB(
				password, host, port, dbName, config.Maxretries, config.PoolSize, config.MinIdleConnections,
				config.DialTimeout, config.ReadTimeout, config.WriteTimeout, config.PoolTimeout,
			)

		} else {
			redisConns[t.TenantID], redisDBNames[t.TenantID] = nil, -1
		}
	}

	return redisConns, redisDBNames
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
