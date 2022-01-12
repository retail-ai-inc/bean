/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package helpers

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"time"

	/**#bean*/
	"demo/framework/dbdrivers"
	/*#bean.replace("{{ .PkgPath }}/framework/dbdrivers")**/
	/**#bean*/
	"demo/framework/internals/global"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/global")**/
	/**#bean*/
	str "demo/framework/internals/string"
	/*#bean.replace(str "{{ .PkgPath }}/framework/internals/string")**/

	"github.com/getsentry/sentry-go"
	"github.com/go-redis/redis/v8"
	"github.com/labstack/echo/v4"
	echomiddleware "github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"
)

type CopyableMap map[string]interface{}
type CopyableSlice []interface{}

// This function mainly used by various commands. So `panicking` here is absolutely OK.
func GetContextInstanceEnvironmentAndConfig() (*echo.Echo, echo.Context, string) {

	// Set viper path and read configuration. You must keep `env.json` file in the root of your project.
	viper.AddConfigPath(".")
	viper.SetConfigType("json")
	viper.SetConfigName("env")

	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	e := echo.New()
	c := e.AcquireContext()
	c.Reset(nil, nil)

	// Get log type (file or stdout) settings from config.
	isLogStdout := viper.GetBool("isLogStdout")

	e.Logger.SetLevel(log.DEBUG)

	if !isLogStdout {

		logFile := viper.GetString("logFile")

		// XXX: IMPORTANT - Set log output into file instead `stdout`.
		logfp, err := os.OpenFile(logFile, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0664)
		if err != nil {
			fmt.Printf("Unable to open log file: %v Exiting...\n", err)
			os.Exit(1)
		}

		e.Logger.SetOutput(logfp)

	} else {
		logger := echomiddleware.LoggerWithConfig(echomiddleware.LoggerConfig{
			Format: JsonLogFormat(), // we need additional access log parameter
		})
		e.Use(logger)
	}

	// Sentry `panic`` error handler initialization if activated from `env.json`
	isSentry := viper.GetBool("sentry.isSentry")
	if isSentry {

		// HTTPSyncTransport is an implementation of `Transport` interface which blocks after each captured event.
		sentrySyncTransport := sentry.NewHTTPSyncTransport()
		sentrySyncTransport.Timeout = time.Second * 30 // Blocks for maximum 30 seconds.

		// To initialize Sentry's handler, we need to initialize Sentry itself beforehand
		err := sentry.Init(sentry.ClientOptions{
			Dsn:              viper.GetString("sentry.dsn"),
			AttachStacktrace: true,
			Transport:        sentrySyncTransport,
		})
		if err != nil {
			panic(err)
		}

		defer sentry.Flush(2 * time.Second)
	}

	// Initialize the global echo instance. This is useful to print log from internal utils packages.
	global.EchoInstance = e
	global.Environment = viper.GetString("environment")

	// Initialize all database driver.
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

	global.DBConn = &global.DBDeps{
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

	return e, c, global.Environment
}

func GetRandomNumberFromRange(min, max int) int {

	rand.Seed(time.Now().UnixNano())

	n := min + rand.Intn(max-min+1)

	return n
}

/* DeepCopy will create a deep copy of this map. The depth of this
 * copy is all inclusive. Both maps and slices will be considered when
 * making the copy.
 *
 * Keep in mind that the slices in the resulting map will be of type []interface{},
 * so when using them, you will need to use type assertion to retrieve the value in the expected type.
 *
 * Reference: https://stackoverflow.com/questions/23057785/how-to-copy-a-map/23058707
 */
func (m CopyableMap) DeepCopy() map[string]interface{} {

	result := map[string]interface{}{}

	for k, v := range m {
		// Handle maps
		mapvalue, isMap := v.(map[string]interface{})
		if isMap {
			result[k] = CopyableMap(mapvalue).DeepCopy()
			continue
		}

		// Handle slices
		slicevalue, isSlice := v.([]interface{})
		if isSlice {
			result[k] = CopyableSlice(slicevalue).DeepCopy()
			continue
		}

		result[k] = v
	}

	return result
}

/* DeepCopy will create a deep copy of this slice. The depth of this
 * copy is all inclusive. Both maps and slices will be considered when
 * making the copy.
 *
 * Reference: https://stackoverflow.com/questions/23057785/how-to-copy-a-map/23058707
 */
func (s CopyableSlice) DeepCopy() []interface{} {
	result := []interface{}{}

	for _, v := range s {
		// Handle maps
		mapvalue, isMap := v.(map[string]interface{})
		if isMap {
			result = append(result, CopyableMap(mapvalue).DeepCopy())
			continue
		}

		// Handle slices
		slicevalue, isSlice := v.([]interface{})
		if isSlice {
			result = append(result, CopyableSlice(slicevalue).DeepCopy())
			continue
		}

		result = append(result, v)
	}

	return result
}

// `IsFilesExistInDirectory` function will check the files (filesToCheck) exist in a specific diretory or not.
func IsFilesExistInDirectory(path string, filesToCheck []string) (bool, error) {

	var matchCount int

	numberOfFileToCheck := len(filesToCheck)

	if numberOfFileToCheck == 0 {
		return false, nil
	}

	files, err := ioutil.ReadDir(path)
	if err != nil {
		return false, err
	}

	for _, file := range files {

		if !file.IsDir() {
			isMatch := str.Contains(filesToCheck, file.Name())
			if isMatch {
				matchCount++
			}
		}
	}

	if numberOfFileToCheck == matchCount {
		return true, nil
	}

	return false, nil
}
