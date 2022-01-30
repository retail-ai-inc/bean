/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package dbdrivers

import (
	"context"
	"encoding/json"
	"time"

	/**#bean*/
	"demo/framework/internals/aes"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/aes")**/

	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"
)

// Init the mongo database connection map.
func InitMongoTenantConns(master *gorm.DB) (map[uint64]*mongo.Client, map[uint64]string) {

	tenantCfgs := GetAllTenantCfgs(master)

	return getAllMongoTenantDB(tenantCfgs)
}

func InitMongoMasterConn() (*mongo.Client, string) {

	masterCfg := viper.GetStringMap("database.mongo.master")

	if len(masterCfg) > 0 {
		userName := masterCfg["username"].(string)
		password := masterCfg["password"].(string)
		host := masterCfg["host"].(string)
		port := masterCfg["port"].(string)
		dbName := masterCfg["database"].(string)

		return connectMongoDB(userName, password, host, port, dbName)
	}

	return nil, ""
}

func getAllMongoTenantDB(tenantCfgs []*TenantCfg) (map[uint64]*mongo.Client, map[uint64]string) {

	mongoConns := make(map[uint64]*mongo.Client, len(tenantCfgs))
	mongoDBNames := make(map[uint64]string, len(tenantCfgs))

	for _, t := range tenantCfgs {

		var cfgsMap map[string]map[string]interface{}

		if t.Connections != nil {
			if err := json.Unmarshal(t.Connections, &cfgsMap); err != nil {
				panic(err)
			}
		}

		mongoCfg := cfgsMap["mongodb"]
		userName := mongoCfg["username"].(string)
		encryptedPassword := mongoCfg["password"].(string)

		// Tenant database password is encrypted in master db config.
		tenantDBPassPhraseKey := viper.GetString("melonpanPassPhraseKey")
		password, err := aes.MelonpanAESDecrypt(tenantDBPassPhraseKey, encryptedPassword)
		if err != nil {
			panic(err)
		}

		host := mongoCfg["host"].(string)
		port := mongoCfg["port"].(string)
		dbName := mongoCfg["database"].(string)

		mongoConns[t.TenantID], mongoDBNames[t.TenantID] = connectMongoDB(userName, password, host, port, dbName)
	}

	return mongoConns, mongoDBNames
}

func connectMongoDB(userName, password, host, port, dbName string) (*mongo.Client, string) {

	connStr := "mongodb://" + host + ":" + port
	timeout := viper.GetDuration("database.mongo.connectTimeout") * time.Second
	credential := options.Credential{
		Username:   userName,
		Password:   password,
		AuthSource: dbName,
	}

	maxConnectionPoolSize := viper.GetUint64("database.mongo.maxConnectionPoolSize")
	maxConnectionLifeTime := viper.GetDuration("database.mongo.maxConnectionLifeTime") * time.Second

	opts := options.Client().
		ApplyURI(connStr).
		SetAuth(credential).
		SetConnectTimeout(timeout).
		SetMaxPoolSize(maxConnectionPoolSize).
		SetMaxConnIdleTime(maxConnectionLifeTime)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mdb, err := mongo.Connect(ctx, opts)
	if err != nil {
		panic(err)
	}

	return mdb, dbName
}

// TODO: call disconnect when the app close.
