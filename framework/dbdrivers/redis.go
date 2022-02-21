// Copyright The RAI Inc.
// The RAI Authors
package dbdrivers

import (
	"encoding/json"

	"github.com/go-redis/redis/v8"
	"github.com/retail-ai-inc/bean/framework/aes"
	"gorm.io/gorm"
)

type RedisConfig struct {
	Master *struct {
		Database int
		Password string
		Host     string
		port     string
	}
	Prefix     string
	Maxretries int
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
		return connectRedisDB(masterCfg.Password, masterCfg.Host, masterCfg.port, masterCfg.Database, config.Maxretries)
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
				password, host, port, dbName, config.Maxretries)

		} else {
			redisConns[t.TenantID], redisDBNames[t.TenantID] = nil, -1
		}
	}

	return redisConns, redisDBNames
}

func connectRedisDB(password, host, port string, dbName int, maxretries int) (*redis.Client, int) {

	rdb := redis.NewClient(&redis.Options{
		Addr:       host + ":" + port,
		Password:   password,
		DB:         dbName,
		MaxRetries: maxretries,
	})

	return rdb, dbName
}

func GetRedisCachePrefix() string {
	return cachePrefix
}
