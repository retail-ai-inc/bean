{{ .Copyright }}
package global

import (
	"github.com/dgraph-io/badger/v3"
	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"
)

// All database connections are initialized using `DBDeps` structure.
type DBDeps struct {
	MasterMySQLDB      *gorm.DB
	MasterMySQLDBName  string
	TenantMySQLDBs     map[uint64]*gorm.DB
	TenantMySQLDBNames map[uint64]string
	MasterMongoDB      *mongo.Client
	MasterMongoDBName  string
	TenantMongoDBs     map[uint64]*mongo.Client
	TenantMongoDBNames map[uint64]string
	MasterRedisDB      *redis.Client
	MasterRedisDBName  int
	TenantRedisDBs     map[uint64]*redis.Client
	TenantRedisDBNames map[uint64]int
	BadgerDB           *badger.DB
}

// We sometimes need echo instance to print log from internal utils packages without passing echo context or instance
var (
	EchoInstance *echo.Echo
	Environment  string
	DBConn       *DBDeps // DBConn will be used by any repository function to connect right database.
)
