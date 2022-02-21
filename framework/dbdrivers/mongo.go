// Copyright The RAI Inc.
// The RAI Authors
package dbdrivers

import (
	"context"
	"encoding/json"
	"time"

	"github.com/retail-ai-inc/bean/framework/aes"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"
)

type MongoConfig struct {
	Master *struct {
		Database string
		Username string
		Password string
		Host     string
		Port     string
	}
	ConnectTimeout        time.Duration
	MaxConnectionPoolSize uint64
	MaxConnectionLifeTime time.Duration
}

// Init the mongo database connection map.
func InitMongoTenantConns(config MongoConfig, master *gorm.DB, tenantDBPassPhraseKey string) (map[uint64]*mongo.Client, map[uint64]string) {

	tenantCfgs := GetAllTenantCfgs(master)

	return getAllMongoTenantDB(config, tenantCfgs, tenantDBPassPhraseKey)
}

func InitMongoMasterConn(config MongoConfig) (*mongo.Client, string) {

	masterCfg := config.Master
	if masterCfg != nil && masterCfg.Database != "" {
		return connectMongoDB(masterCfg.Username, masterCfg.Password, masterCfg.Host, masterCfg.Port, masterCfg.Database,
			config.MaxConnectionPoolSize, config.ConnectTimeout, config.MaxConnectionLifeTime)
	}

	return nil, ""
}

func getAllMongoTenantDB(config MongoConfig, tenantCfgs []*TenantConnections, tenantDBPassPhraseKey string) (map[uint64]*mongo.Client, map[uint64]string) {

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
			if tenantDBPassPhraseKey != "" {
				password, err = aes.BeanAESDecrypt(tenantDBPassPhraseKey, password)
				if err != nil {
					panic(err)
				}
			}

			host := mongoCfg["host"].(string)
			port := mongoCfg["port"].(string)
			dbName := mongoCfg["database"].(string)

			mongoConns[t.TenantID], mongoDBNames[t.TenantID] = connectMongoDB(
				userName, password, host, port, dbName, config.MaxConnectionPoolSize,
				config.ConnectTimeout, config.MaxConnectionLifeTime)

		} else {
			mongoConns[t.TenantID], mongoDBNames[t.TenantID] = nil, ""
		}
	}

	return mongoConns, mongoDBNames
}

func connectMongoDB(userName, password, host, port, dbName string, maxConnectionPoolSize uint64,
	connectTimeout, maxConnectionLifeTime time.Duration) (*mongo.Client, string) {

	connStr := "mongodb://" + host + ":" + port

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Client().
		ApplyURI(connStr).
		SetConnectTimeout(connectTimeout).
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
