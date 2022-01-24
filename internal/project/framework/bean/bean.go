/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package bean

import (
	"net/http"
	/**#bean*/
	"demo/framework/kernel"
	/*#bean.replace("{{ .PkgPath }}/framework/kernel")**/
	/**#bean*/
	"demo/framework/dbdrivers"
	/*#bean.replace("{{ .PkgPath }}/framework/dbdrivers")**/
	/**#bean*/
	berror "demo/framework/internals/error"
	/*#bean.replace(berror "{{ .PkgPath }}/framework/internals/error")**/
	/**#bean*/
	"demo/framework/internals/helpers"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/helpers")**/
	/**#bean*/
	validate "demo/framework/internals/validator"
	/*#bean.replace(validate "{{ .PkgPath }}/framework/internals/validator")**/

	"github.com/dgraph-io/badger/v3"
	"github.com/go-playground/validator/v10"
	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	"github.com/spf13/viper"
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

type Bean struct {
	DBConn                  *DBDeps
	Echo                    *echo.Echo
	Environment             string
	Validate                func(c echo.Context, vd *validator.Validate)
	BeforeServe             func()
	errorHandlerMiddlewares []berror.ErrorHandlerMiddleware
}

func New() (b *Bean) {
	// Parse bean system files and directories.
	helpers.ParseBeanSystemFilesAndDirectorires()

	// Create a new echo instance
	e := kernel.NewEcho()

	b = &Bean{
		Echo: e,
	}

	return b
}

func (b *Bean) ServeAt(host, port string) {
	// before bean bootstrap
	if b.BeforeServe != nil {
		b.BeforeServe()
	}

	b.UseErrorHandlerMiddleware(
		berror.ValidationErrorHanderMiddleware,
		berror.APIErrorHanderMiddleware,
		berror.HTTPErrorHanderMiddleware,
		berror.DefaultErrorHanderMiddleware,
	)

	b.Echo.HTTPErrorHandler = berror.ErrorHandlerChain(b.errorHandlerMiddlewares...)

	// Initialize and bind the validator to echo instance
	validate.BindCustomValidator(b.Echo, b.Validate)

	projectName := viper.GetString("name")

	b.Echo.Logger.Info(`Starting ` + projectName + ` server...🚀`)

	s := http.Server{
		Addr:    host + ":" + port,
		Handler: b.Echo,
	}

	// IMPORTANT: Keep-alive is default true but I kept this here to let you guys no that there is a settings
	// for it :)
	s.SetKeepAlivesEnabled(viper.GetBool("http.keepAlive"))

	// Start the server
	if err := s.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		b.Echo.Logger.Fatal(err)
	}
}

func (b *Bean) UseErrorHandlerMiddleware(errorHandlerMiddleware ...berror.ErrorHandlerMiddleware) {
	if b.errorHandlerMiddlewares == nil {
		b.errorHandlerMiddlewares = []berror.ErrorHandlerMiddleware{}
	}
	b.errorHandlerMiddlewares = append(b.errorHandlerMiddlewares, errorHandlerMiddleware...)
}

// InitDB initialize all the database dependencies and store it in global variable `global.DBConn`.
func (b *Bean) InitDB() {
	var masterMySQLDB *gorm.DB
	var masterMySQLDBName string
	var masterMongoDB *mongo.Client
	var masterMongoDBName string
	var masterRedisDB *redis.Client
	var masterRedisDBName int

	var tenantMySQLDBs map[uint64]*gorm.DB
	var tenantMySQLDBNames map[uint64]string
	var tenantMongoDBs map[uint64]*mongo.Client
	var tenantMongoDBNames map[uint64]string
	var tenantRedisDBs map[uint64]*redis.Client
	var tenantRedisDBNames map[uint64]int

	isTenant := viper.GetBool("database.mysql.isTenant")
	if isTenant {
		masterMySQLDB, masterMySQLDBName = dbdrivers.InitMysqlMasterConn()
		tenantMySQLDBs, tenantMySQLDBNames = dbdrivers.InitMysqlTenantConns(masterMySQLDB)
		tenantMongoDBs, tenantMongoDBNames = dbdrivers.InitMongoTenantConns(masterMySQLDB)
		tenantRedisDBs, tenantRedisDBNames = dbdrivers.InitRedisTenantConns(masterMySQLDB)
	} else {
		masterMySQLDB, masterMySQLDBName = dbdrivers.InitMysqlMasterConn()
		masterMongoDB, masterMongoDBName = dbdrivers.InitMongoMasterConn()
		masterRedisDB, masterRedisDBName = dbdrivers.InitRedisMasterConn()
	}

	masterBadgerDB := dbdrivers.InitBadgerConn()

	b.DBConn = &DBDeps{
		MasterMySQLDB:      masterMySQLDB,
		MasterMySQLDBName:  masterMySQLDBName,
		TenantMySQLDBs:     tenantMySQLDBs,
		TenantMySQLDBNames: tenantMySQLDBNames,
		MasterMongoDB:      masterMongoDB,
		MasterMongoDBName:  masterMongoDBName,
		TenantMongoDBs:     tenantMongoDBs,
		TenantMongoDBNames: tenantMongoDBNames,
		MasterRedisDB:      masterRedisDB,
		MasterRedisDBName:  masterRedisDBName,
		TenantRedisDBs:     tenantRedisDBs,
		TenantRedisDBNames: tenantRedisDBNames,
		BadgerDB:           masterBadgerDB,
	}
}
