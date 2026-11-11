// MIT License

// Copyright (c) The RAI Authors

// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:

// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.

// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

package dbdrivers

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"time"

	mysqlDriver "github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"github.com/retail-ai-inc/bean/v2/aes"
	"gorm.io/datatypes"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormLogger "gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"
)

const mysqlReplicaPingTimeout = 5 * time.Second

type SQLConfig struct {
	Master *struct {
		Database string
		Username string
		Password string
		Host     string
		Port     string
		// Reads lists read replicas as `host` or `host:port`; they share the master credentials,
		// database and SSL settings, and use the master port when none is given.
		Reads []string
		SSL   SSLConfig
	}
	MaxIdleConnections        int
	MaxOpenConnections        int
	MaxConnectionLifeTime     time.Duration
	MaxIdleConnectionLifeTime time.Duration
	Debug                     bool
}

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

var (
	noClose   = func() error { return nil }
	noClosers = []func() error{}
)

// InitMysqlMasterConn returns mysql master db connection.
func InitMysqlMasterConn(config SQLConfig, logger echo.Logger) (*gorm.DB, string, func() error, error) {

	masterCfg := config.Master

	if masterCfg != nil && masterCfg.Database != "" {
		return connectMysqlDB(
			masterCfg.Username, masterCfg.Password, masterCfg.Host, masterCfg.Port, masterCfg.Database, masterCfg.Reads,
			config.MaxIdleConnections, config.MaxOpenConnections, config.MaxConnectionLifeTime, config.MaxIdleConnectionLifeTime,
			config.Debug, masterCfg.SSL, logger,
		)
	}

	return nil, "", noClose, nil
}

func InitMysqlTenantConns(config SQLConfig, master *gorm.DB, tenantAlterDbHostParam, tenantDBPassPhraseKey string, logger echo.Logger,
) (map[uint64]*gorm.DB, map[uint64]string, []func() error, error) {

	err := createTenantConnectionsTableIfNotExist(master)
	if err != nil {
		return nil, nil, noClosers, err
	}

	tenantCfgs, err := GetAllTenantCfgs(master)
	if err != nil {
		return nil, nil, noClosers, err
	}

	return getAllMysqlTenantDB(config, tenantCfgs, tenantAlterDbHostParam, tenantDBPassPhraseKey, logger)
}

// GetAllTenantCfgs return all Tenant data from master db.
// Prevent multiple calls
var tts []*TenantConnections

func GetAllTenantCfgs(db *gorm.DB) ([]*TenantConnections, error) {

	if len(tts) > 0 {
		return tts, nil
	}
	var tt []*TenantConnections
	err := db.Table("TenantConnections").Find(&tt).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get all tenant connections: %w", err)
	}
	// TODO: save the config in memory
	tts = slices.Clone(tt)

	return tt, nil
}

// getAllMysqlTenantDB returns all tenant db connection.
func getAllMysqlTenantDB(config SQLConfig, tenantCfgs []*TenantConnections,
	tenantAlterDbHostParam, tenantDBPassPhraseKey string, logger echo.Logger,
) (mysqlConns map[uint64]*gorm.DB, mysqlDBNames map[uint64]string, resultClosers []func() error, err error) {

	mysqlConns = make(map[uint64]*gorm.DB, len(tenantCfgs))
	mysqlDBNames = make(map[uint64]string, len(tenantCfgs))
	openedClosers := make([]func() error, 0, len(tenantCfgs))

	for _, t := range tenantCfgs {

		var cfgsMap map[string]map[string]interface{}
		if t.Connections != nil {
			if err = json.Unmarshal(t.Connections, &cfgsMap); err != nil {
				return nil, nil, nil, fmt.Errorf("failed to unmarshal tenant connections (%d:%s): %w", t.TenantID, t.Code, err)
			}
		}

		// IMPORTANT: Check the `mysql` object exist in the Connections column or not.
		mysqlCfg, exists := cfgsMap["mysql"]
		if !exists {
			mysqlConns[t.TenantID], mysqlDBNames[t.TenantID] = nil, ""
			continue
		}
		userName := mysqlCfg["username"].(string)
		password := mysqlCfg["password"].(string)

		// IMPORTANT: If tenant database password is encrypted in master db config.
		if tenantDBPassPhraseKey != "" {
			password, err = aes.BeanAESDecrypt(tenantDBPassPhraseKey, password)
			if err != nil {
				return nil, nil, nil, fmt.Errorf("failed to decrypt mysql tenant database password (%d:%s): %w", t.TenantID, t.Code, err)
			}
		}

		host := mysqlCfg["host"].(string)

		// IMPORTANT - If a command or service wants to use a different `host` parameter for tenant database connection
		// then it's easy to do just by passing that parameter string name using `bean.TenantAlterDbHostParam`.
		// Therfore, `bean` will overwrite all host string in `TenantConnections`.`Connections` JSON.
		if tenantAlterDbHostParam != "" && mysqlCfg[tenantAlterDbHostParam] != nil {
			host = mysqlCfg[tenantAlterDbHostParam].(string)
		}

		port := mysqlCfg["port"].(string)
		dbName := mysqlCfg["database"].(string)
		ssl := sslConfigFromMap(mysqlCfg)

		readHosts, err := readHostsFromMap(mysqlCfg)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("invalid mysql tenant read replicas (%d:%s): %w", t.TenantID, t.Code, err)
		}

		var closeDB func() error
		mysqlConns[t.TenantID], mysqlDBNames[t.TenantID], closeDB, err = connectMysqlDB(
			userName, password, host, port, dbName, readHosts, config.MaxIdleConnections,
			config.MaxOpenConnections, config.MaxConnectionLifeTime, config.MaxIdleConnectionLifeTime,
			config.Debug, ssl, logger,
		)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to connect mysql tenant database (%d:%s): %w", t.TenantID, t.Code, err)
		}
		openedClosers = append(openedClosers, closeDB)
	}

	return mysqlConns, mysqlDBNames, openedClosers, nil
}

func connectMysqlDB(userName, password, host, port, dbName string, readHosts []string,
	maxIdleConnections, maxOpenConnections int, maxConnectionLifeTime, maxIdleConnectionLifeTime time.Duration,
	debug bool, ssl SSLConfig, logger echo.Logger) (*gorm.DB, string, func() error, error) {

	readAddrs, err := parseReadAddrs(readHosts, port)
	if err != nil {
		return nil, "", noClose, err
	}

	dsn, err := mysqlDSN(userName, password, host, port, dbName, ssl)
	if err != nil {
		return nil, "", noClose, err
	}

	logMode := gormLogger.Silent
	if debug {
		logMode = gormLogger.Info
	}

	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: gormLogger.Default.LogMode(logMode)})
	if err != nil {
		return nil, "", noClose, fmt.Errorf("failed to connect mysql database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, "", noClose, fmt.Errorf("failed to get mysql database connection: %w", err)
	}

	setMysqlConnPool(sqlDB, maxIdleConnections, maxOpenConnections, maxConnectionLifeTime, maxIdleConnectionLifeTime)
	closers := []func() error{sqlDB.Close}

	if len(readAddrs) > 0 {
		replicas := make([]gorm.Dialector, 0, len(readAddrs))
		for _, addr := range readAddrs {
			readHost := net.JoinHostPort(addr.host, addr.port)
			replicaDSN, err := mysqlDSN(userName, password, addr.host, addr.port, dbName, ssl)
			if err != nil {
				return nil, "", noClose, errors.Join(err, closeAll(closers))
			}

			replicaDB, err := sql.Open("mysql", replicaDSN)
			if err != nil {
				return nil, "", noClose, errors.Join(fmt.Errorf("failed to open mysql read replica %s: %w", readHost, err), closeAll(closers))
			}

			setMysqlConnPool(replicaDB, maxIdleConnections, maxOpenConnections, maxConnectionLifeTime, maxIdleConnectionLifeTime)
			closers = append(closers, replicaDB.Close)

			// An unreachable replica must not block startup; configuration errors such as bad credentials still do.
			pool := newReplicaConnPool(readHost, replicaDB, sqlDB, logger)
			pingCtx, cancel := context.WithTimeout(context.Background(), mysqlReplicaPingTimeout)
			pingErr := replicaDB.PingContext(pingCtx)
			cancel()
			if pingErr != nil {
				if errors.Is(pingErr, context.DeadlineExceeded) {
					pool.setUnavailable(pingErr)
				} else if !pool.markUnavailable(pingErr) {
					return nil, "", noClose, errors.Join(fmt.Errorf("failed to connect mysql read replica %s: %w", readHost, pingErr), closeAll(closers))
				}
			}

			replicas = append(replicas, mysql.New(mysql.Config{DSN: replicaDSN, Conn: pool, SkipInitializeWithVersion: true}))
		}

		if err := useMysqlReplicas(db, replicas); err != nil {
			return nil, "", noClose, errors.Join(err, closeAll(closers))
		}
	}

	return db, dbName, func() error { return closeAll(closers) }, nil
}

// useMysqlReplicas routes reads outside transactions to a random replica;
// writes, transactions and locking reads stay on the primary.
func useMysqlReplicas(db *gorm.DB, replicas []gorm.Dialector) error {
	if len(replicas) == 0 {
		return nil
	}

	if err := db.Use(dbresolver.Register(dbresolver.Config{Replicas: replicas, Policy: dbresolver.RandomPolicy{}})); err != nil {
		return fmt.Errorf("failed to register mysql read replicas: %w", err)
	}

	return nil
}

func mysqlDSN(userName, password, host, port, dbName string, ssl SSLConfig) (string, error) {
	tlsConfigName := ""
	if ssl.On {
		tlsConfig, err := newTLSConfig(ssl)
		if err != nil {
			return "", err
		}

		tlsConfigName = mysqlTLSConfigName(host, dbName, ssl)
		if err := mysqlDriver.RegisterTLSConfig(tlsConfigName, tlsConfig); err != nil {
			return "", fmt.Errorf("failed to register mysql tls config: %w", err)
		}
	}

	return fmt.Sprintf(
		"%s:%s@tcp(%s)/%s?parseTime=true&multiStatements=true%s",
		userName, password, net.JoinHostPort(host, port), dbName,
		mysqlTLSDSNParam(tlsConfigName),
	), nil
}

type mysqlAddr struct {
	host string
	port string
}

// parseReadAddrs accepts `host` or `host:port` entries (IPv6 as `[::1]:3307`);
// entries without a port use defaultPort.
func parseReadAddrs(readHosts []string, defaultPort string) ([]mysqlAddr, error) {
	addrs := make([]mysqlAddr, 0, len(readHosts))
	for i, readHost := range readHosts {
		addr, err := parseReadAddr(readHost, defaultPort)
		if err != nil {
			return nil, fmt.Errorf("invalid mysql read replica at index %d: %w", i, err)
		}
		addrs = append(addrs, addr)
	}

	return addrs, nil
}

func parseReadAddr(readHost, defaultPort string) (mysqlAddr, error) {
	host, port, err := net.SplitHostPort(readHost)
	if err != nil {
		var addrErr *net.AddrError
		if !errors.As(err, &addrErr) || addrErr.Err != "missing port in address" {
			return mysqlAddr{}, fmt.Errorf("%q: %w", readHost, err)
		}
		host, port = strings.TrimSuffix(strings.TrimPrefix(readHost, "["), "]"), defaultPort
	}

	if host == "" {
		return mysqlAddr{}, fmt.Errorf("%q: host is empty", readHost)
	}

	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return mysqlAddr{}, fmt.Errorf("%q: invalid port %q", readHost, port)
	}

	return mysqlAddr{host: host, port: port}, nil
}

func setMysqlConnPool(sqlDB *sql.DB, maxIdleConnections, maxOpenConnections int, maxConnectionLifeTime, maxIdleConnectionLifeTime time.Duration) {
	sqlDB.SetMaxIdleConns(maxIdleConnections)
	sqlDB.SetMaxOpenConns(maxOpenConnections)

	if maxConnectionLifeTime > 0 {
		sqlDB.SetConnMaxLifetime(maxConnectionLifeTime)
	}

	if maxIdleConnectionLifeTime > 0 {
		sqlDB.SetConnMaxIdleTime(maxIdleConnectionLifeTime)
	}
}

func closeAll(closers []func() error) error {
	var errs []error
	for _, closeFn := range closers {
		if err := closeFn(); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// readHostsFromMap returns the `reads` host list of a tenant connection config.
func readHostsFromMap(cfg map[string]interface{}) ([]string, error) {
	raw, ok := cfg["reads"]
	if !ok || raw == nil {
		return nil, nil
	}

	items, ok := raw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("reads must be an array of hosts, got %T", raw)
	}

	hosts := make([]string, 0, len(items))
	for i, item := range items {
		host, ok := item.(string)
		if !ok || host == "" {
			return nil, fmt.Errorf("reads[%d] must be a non-empty host string", i)
		}
		hosts = append(hosts, host)
	}

	return hosts, nil
}

func mysqlTLSDSNParam(tlsConfigName string) string {
	if tlsConfigName == "" {
		return ""
	}

	return "&tls=" + tlsConfigName
}

func mysqlTLSConfigName(host, dbName string, ssl SSLConfig) string {
	key := fmt.Sprintf("%s|%s|%s|%t|%t", host, dbName, ssl.CertFile, ssl.ShouldVerifyCertificate(), ssl.HotReload)
	sum := sha256.Sum256([]byte(key))

	return fmt.Sprintf("bean-%x", sum[:8])
}

func createTenantConnectionsTableIfNotExist(masterDb *gorm.DB) error {

	if masterDb.Migrator().HasTable("TenantConnections") {
		return nil
	}

	if err := masterDb.Migrator().CreateTable(&TenantConnections{}); err != nil {
		return fmt.Errorf("failed to create TenantConnections table: %w", err)
	}

	return nil
}
