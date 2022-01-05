/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package dbdrivers

import (
	"encoding/json"

	/**#bean*/
	"demo/framework/internals/aes"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/aes")**/

	"github.com/go-redis/redis/v8"
	"github.com/spf13/viper"
	"gorm.io/gorm"
)

var cachePrefix string

func InitRedisTenantConns(master *gorm.DB) (map[uint64]*redis.Client, map[uint64]int) {

	cachePrefix = viper.GetString("database.redis.prefix")
	tenantCfgs := GetAllTenantCfgs(master)

	return getAllRedisTenantDB(tenantCfgs)
}

func InitRedisMasterConn() (*redis.Client, int) {

	masterCfg := viper.GetStringMap("database.redis.master")

	if len(masterCfg) > 0 {
		password := masterCfg["password"].(string)
		host := masterCfg["host"].(string)
		port := masterCfg["port"].(string)
		dbName := masterCfg["database"].(int)

		return connectRedisDB(password, host, port, dbName)
	}

	return nil, -1
}

// GetTenantDB returns a singleton tenant db connection.
func getAllRedisTenantDB(tenantCfgs []*TenantCfg) (map[uint64]*redis.Client, map[uint64]int) {

	redisConns := make(map[uint64]*redis.Client, len(tenantCfgs))
	redisDBNames := make(map[uint64]int, len(tenantCfgs))

	for _, t := range tenantCfgs {

		var cfgsMap map[string]map[string]interface{}

		if t.Connections != nil {
			if err := json.Unmarshal(t.Connections, &cfgsMap); err != nil {
				panic(err)
			}
		}

		redisCfg := cfgsMap["redis"]
		encryptedPassword := redisCfg["password"].(string)

		// Tenant database password is encrypted in master db config.
		tenantDBPassPhraseKey := viper.GetString("melonpanPassPhraseKey")
		password, err := aes.MelonpanAESDecrypt(tenantDBPassPhraseKey, encryptedPassword)
		if err != nil {
			panic(err)
		}

		host := redisCfg["host"].(string)
		port := redisCfg["port"].(string)
		dbName := viper.GetInt("database.redis.database")

		redisConns[t.TenantID], redisDBNames[t.TenantID] = connectRedisDB(password, host, port, dbName)
	}

	return redisConns, redisDBNames
}

func connectRedisDB(password, host, port string, dbName int) (*redis.Client, int) {

	rdb := redis.NewClient(&redis.Options{
		Addr:       host + ":" + port,
		Password:   password,
		DB:         dbName,
		MaxRetries: viper.GetInt("database.redis.maxretries"),
	})

	return rdb, dbName
}

func GetRedisCachePrefix() string {
	return cachePrefix
}
