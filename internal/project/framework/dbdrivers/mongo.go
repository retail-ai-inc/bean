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
	database := masterCfg["database"].(string)
	if len(masterCfg) > 0 && database != "" {
		userName := masterCfg["username"].(string)
		password := masterCfg["password"].(string)
		host := masterCfg["host"].(string)
		port := masterCfg["port"].(string)
		dbName := database

		return connectMongoDB(userName, password, host, port, dbName)
	}

	return nil, ""
}

func getAllMongoTenantDB(tenantCfgs []*TenantConnections) (map[uint64]*mongo.Client, map[uint64]string) {

	mongoConns := make(map[uint64]*mongo.Client, len(tenantCfgs))
	mongoDBNames := make(map[uint64]string, len(tenantCfgs))

	for _, t := range tenantCfgs {

		var cfgsMap map[string]map[string]interface{}
		var err error
		if t.Connections != nil {
			if err = json.Unmarshal(t.Connections, &cfgsMap); err != nil {
				panic(err)
			}
		}

		// IMPORTANT: Check the `mongodb` object exist in the Connections column or not.
		if mongoCfg, ok := cfgsMap["mongodb"]; ok {
			userName := mongoCfg["username"].(string)
			password := mongoCfg["password"].(string)

			// IMPORTANT: If tenant database password is encrypted in master db config.
			tenantDBPassPhraseKey := viper.GetString("database.tenant.secret")
			if tenantDBPassPhraseKey != "" {
				password, err = aes.MelonpanAESDecrypt(tenantDBPassPhraseKey, password)
				if err != nil {
					panic(err)
				}
			}

			host := mongoCfg["host"].(string)
			port := mongoCfg["port"].(string)
			dbName := mongoCfg["database"].(string)

			mongoConns[t.TenantID], mongoDBNames[t.TenantID] = connectMongoDB(userName, password, host, port, dbName)

		} else {
			mongoConns[t.TenantID], mongoDBNames[t.TenantID] = nil, ""
		}
	}

	return mongoConns, mongoDBNames
}

func connectMongoDB(userName, password, host, port, dbName string) (*mongo.Client, string) {

	connStr := "mongodb://" + host + ":" + port
	timeout := viper.GetDuration("database.mongo.connectTimeout")
	maxConnectionPoolSize := viper.GetUint64("database.mongo.maxConnectionPoolSize")
	maxConnectionLifeTime := viper.GetDuration("database.mongo.maxConnectionLifeTime")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Client().
		ApplyURI(connStr).
		SetConnectTimeout(timeout).
		SetMaxPoolSize(maxConnectionPoolSize).
		SetMaxConnIdleTime(maxConnectionLifeTime)

	if userName != "" && password != "" {
		credential := options.Credential{Username: userName, Password: password, AuthSource: dbName}
		opts.SetAuth(credential)
	}

	mdb, err := mongo.Connect(ctx, opts)
	if err != nil {
		panic(err)
	}

	return mdb, dbName
}
