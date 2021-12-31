/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package dbdrivers

import (
	"encoding/json"
	"fmt"

	/**#bean*/
	"demo/framework/internals/aes" /*#bean.replace("{{ .PkgPath }}/framework/internals/aes")**/

	"github.com/spf13/viper"
	"gorm.io/datatypes"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Tenant represent a tenant database configuration record in masterdatabase
type TenantCfg struct {
	ID          uint64         `gorm:"primary_key;AUTO_INCREMENT;column:Id"`
	UUID        string         `gorm:"column:Uuid"`
	TenantID    uint64         `gorm:"column:RetailerCompanyId"`
	Code        string         `gorm:"column:Code"`
	Connections datatypes.JSON `gorm:"column:Connections"`
}

// InitMysqlMasterConn returns mysql master db connection.
func InitMysqlMasterConn() (*gorm.DB, string) {

	mysqlConfig := viper.GetStringMapString("database.mysql.master")

	if len(mysqlConfig) > 0 {
		userName := mysqlConfig["username"]
		password := mysqlConfig["password"]
		host := mysqlConfig["host"]
		port := mysqlConfig["port"]
		dbName := mysqlConfig["database"]

		return connectMysqlDB(userName, password, host, port, dbName)
	}

	return nil, ""
}

func InitMysqlTenantConns(master *gorm.DB) (map[uint64]*gorm.DB, map[uint64]string) {

	tenantCfgs := GetAllTenantCfgs(master)

	return getAllMysqlTenantDB(tenantCfgs, false)
}

// GetAllTenantCfgs return all Tenant data from master db.
func GetAllTenantCfgs(db *gorm.DB) []*TenantCfg {

	var tt []*TenantCfg

	err := db.Table("TenantConnections").Find(&tt).Error
	if err != nil {
		panic(err)
	}

	// TODO: save the config in badger

	return tt
}

// getAllMysqlTenantDB returns all tenant db connection.
func getAllMysqlTenantDB(tenantCfgs []*TenantCfg, isCloudFunction bool) (map[uint64]*gorm.DB, map[uint64]string) {

	mysqlConns := make(map[uint64]*gorm.DB, len(tenantCfgs))
	mysqlDBNames := make(map[uint64]string, len(tenantCfgs))

	for _, t := range tenantCfgs {

		var cfgsMap map[string]map[string]interface{}

		if t.Connections != nil {
			if err := json.Unmarshal(t.Connections, &cfgsMap); err != nil {
				panic(err)
			}
		}

		mysqlCfg := cfgsMap["mysql"]
		userName := mysqlCfg["username"].(string)
		encryptedPassword := mysqlCfg["password"].(string)

		// Tenant database password is encrypted in master db config.
		tenantDBPassPhraseKey := viper.GetString("melonpanPassPhraseKey")
		password, err := aes.MelonpanAESDecrypt(tenantDBPassPhraseKey, encryptedPassword)
		if err != nil {
			panic(err)
		}

		host := mysqlCfg["host"].(string)

		// Cloud function is running from a different default network.
		if isCloudFunction {
			host = mysqlCfg["gcpHost"].(string)
		}

		port := mysqlCfg["port"].(string)
		dbName := mysqlCfg["database"].(string)

		mysqlConns[t.TenantID], mysqlDBNames[t.TenantID] = connectMysqlDB(userName, password, host, port, dbName)
	}

	return mysqlConns, mysqlDBNames
}

func connectMysqlDB(userName, password, host, port, dbName string) (*gorm.DB, string) {

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true",
		userName, password, host, port, dbName,
	)

	// TODO: use default log mode for stg & prod env
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Info)})
	if err != nil {
		panic(err)
	}

	sqlDB, err := db.DB()
	sqlDB.SetMaxIdleConns(viper.GetInt("database.mysql.maxIdleConnections"))
	sqlDB.SetMaxOpenConns(viper.GetInt("database.mysql.maxOpenConnections"))

	return db, dbName
}
