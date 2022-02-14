/**#bean*/ /*#bean.replace({{ .Copyright }})**/
package dbdrivers

import (
	"encoding/json"
	"fmt"
	"time"

	/**#bean*/
	"demo/framework/internals/aes"
	/*#bean.replace("{{ .PkgPath }}/framework/internals/aes")**/

	"github.com/spf13/viper"
	"gorm.io/datatypes"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// TenantConnections represent a tenant database configuration record in master database
type TenantConnections struct {
	ID          uint64         `gorm:"primary_key;AUTO_INCREMENT;column:Id"`
	UUID        string         `gorm:"type:CHAR(36);not null;unique;column:Uuid"`
	TenantID    uint64         `gorm:"not null;column:TenantId"`
	Code        string         `gorm:"type:VARCHAR(20);not null;unique;column:Code"`
	Connections datatypes.JSON `gorm:"not null;column:Connections"`
	CreatedBy   uint64         `gorm:"not null;default:0;column:CreatedBy"`
	UpdatedBy   uint64         `gorm:"not null;default:0;column:UpdatedBy"`
	DeletedBy   uint64         `gorm:"default:NULL;column:DeletedBy"`
	CreatedAt   time.Time      `gorm:"type:timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP;column:CreatedAt"`
	UpdatedAt   time.Time      `gorm:"type:timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP;column:UpdatedAt"`
	DeletedAt   gorm.DeletedAt `gorm:"type:timestamp NULL DEFAULT NULL;column:DeletedAt"`
}

func (TenantConnections) TableName() string {

	return "TenantConnections"
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

	err := createTenantConnectionsTableIfNotExist(master)
	if err != nil {
		panic(err)
	}

	tenantCfgs := GetAllTenantCfgs(master)

	return getAllMysqlTenantDB(tenantCfgs, false)
}

// GetAllTenantCfgs return all Tenant data from master db.
func GetAllTenantCfgs(db *gorm.DB) []*TenantConnections {

	var tt []*TenantConnections

	err := db.Table("TenantConnections").Find(&tt).Error
	if err != nil {
		panic(err)
	}

	// TODO: save the config in badger

	return tt
}

// getAllMysqlTenantDB returns all tenant db connection.
func getAllMysqlTenantDB(tenantCfgs []*TenantConnections, isCloudFunction bool) (map[uint64]*gorm.DB, map[uint64]string) {

	mysqlConns := make(map[uint64]*gorm.DB, len(tenantCfgs))
	mysqlDBNames := make(map[uint64]string, len(tenantCfgs))

	for _, t := range tenantCfgs {

		var cfgsMap map[string]map[string]interface{}
		var err error
		if t.Connections != nil {
			if err = json.Unmarshal(t.Connections, &cfgsMap); err != nil {
				panic(err)
			}
		}

		// IMPORTANT: Check the `mysql` object exist in the Connections column or not.
		if mysqlCfg, ok := cfgsMap["mysql"]; ok {
			userName := mysqlCfg["username"].(string)
			password := mysqlCfg["password"].(string)

			// IMPORTANT: If tenant database password is encrypted in master db config.
			tenantDBPassPhraseKey := viper.GetString("database.tenant.secret")
			if tenantDBPassPhraseKey != "" {
				password, err = aes.MelonpanAESDecrypt(tenantDBPassPhraseKey, password)
				if err != nil {
					panic(err)
				}
			}

			host := mysqlCfg["host"].(string)

			// Cloud function is running from a different default network.
			if isCloudFunction {
				host = mysqlCfg["gcpHost"].(string)
			}

			port := mysqlCfg["port"].(string)
			dbName := mysqlCfg["database"].(string)

			mysqlConns[t.TenantID], mysqlDBNames[t.TenantID] = connectMysqlDB(userName, password, host, port, dbName)

		} else {
			mysqlConns[t.TenantID], mysqlDBNames[t.TenantID] = nil, ""
		}
	}

	return mysqlConns, mysqlDBNames
}

func connectMysqlDB(userName, password, host, port, dbName string) (*gorm.DB, string) {

	dsn := fmt.Sprintf(
		"%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true",
		userName, password, host, port, dbName,
	)

	var db *gorm.DB
	var err error

	debug := viper.GetBool("database.mysql.debug")
	if debug {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Info)})
	} else {
		db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	}
	if err != nil {
		panic(err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		panic(err)
	}

	sqlDB.SetMaxIdleConns(viper.GetInt("database.mysql.maxIdleConnections"))
	sqlDB.SetMaxOpenConns(viper.GetInt("database.mysql.maxOpenConnections"))

	maxConnectionLifeTime := viper.GetDuration("database.mysql.maxConnectionLifeTime")
	if maxConnectionLifeTime > 0 {
		sqlDB.SetConnMaxLifetime(maxConnectionLifeTime * time.Second)
	}

	return db, dbName
}
func createTenantConnectionsTableIfNotExist(masterDb *gorm.DB) error {

	if !masterDb.Migrator().HasTable("TenantConnections") {
		err := masterDb.Migrator().CreateTable(&TenantConnections{})
		return err
	}

	return nil
}
