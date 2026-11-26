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
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/retail-ai-inc/bean/v3/aes"
	"go.mongodb.org/mongo-driver/event"
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
		SSL      SSLConfig
	}
	ConnectTimeout        time.Duration
	MaxConnectionPoolSize uint64
	MinConnectionPoolSize uint64
	MaxConnectionLifeTime time.Duration
	Debug                 bool
}

// Init the mongo database connection map.
func InitMongoTenantConns(config MongoConfig, master *gorm.DB, tenantAlterDbHostParam, tenantDBPassPhraseKey string, logger *slog.Logger,
) (map[uint64]*mongo.Client, map[uint64]string, []func() error, error) {

	tenantCfgs, err := GetAllTenantCfgs(master)
	if err != nil {
		return nil, nil, noClosers, err
	}

	return getAllMongoTenantDB(config, tenantCfgs, tenantAlterDbHostParam, tenantDBPassPhraseKey, logger)
}

func InitMongoMasterConn(config MongoConfig, logger *slog.Logger) (*mongo.Client, string, func() error, error) {

	masterCfg := config.Master
	if masterCfg != nil && masterCfg.Database != "" {
		return connectMongoDB(masterCfg.Username, masterCfg.Password, masterCfg.Host, masterCfg.Port, masterCfg.Database,
			config.MaxConnectionPoolSize, config.MinConnectionPoolSize,
			config.ConnectTimeout, config.MaxConnectionLifeTime,
			config.Debug, masterCfg.SSL, logger,
		)
	}

	return nil, "", noClose, nil
}

// getAllMongoTenantDB returns all tenant db connections. On error, the connections already opened are closed.
func getAllMongoTenantDB(config MongoConfig, tenantCfgs []*TenantConnections, tenantAlterDbHostParam, tenantDBPassPhraseKey string, logger *slog.Logger,
) (map[uint64]*mongo.Client, map[uint64]string, []func() error, error) {

	mongoConns := make(map[uint64]*mongo.Client, len(tenantCfgs))
	mongoDBNames := make(map[uint64]string, len(tenantCfgs))

	closers := make([]func() error, 0, len(tenantCfgs))
	fail := func(t *TenantConnections, msg string, err error) (map[uint64]*mongo.Client, map[uint64]string, []func() error, error) {
		return nil, nil, noClosers, errors.Join(
			fmt.Errorf("%s (%d:%s): %w", msg, t.TenantID, t.Code, err), closeAll(closers))
	}

	for _, t := range tenantCfgs {

		var cfgsMap map[string]map[string]interface{}
		if t.Connections != nil {
			if err := json.Unmarshal(t.Connections, &cfgsMap); err != nil {
				return fail(t, "failed to unmarshal tenant connections", err)
			}
		}

		// IMPORTANT: Check the `mongodb` object exist in the Connections column or not.
		mongoCfg, exists := cfgsMap["mongodb"]
		if !exists {
			mongoConns[t.TenantID], mongoDBNames[t.TenantID] = nil, ""
			continue
		}

		var userName, password, port, dbName string
		var err error
		for _, f := range []struct {
			dst *string
			key string
		}{{&userName, "username"}, {&password, "password"}, {&port, "port"}, {&dbName, "database"}} {
			if *f.dst, err = requiredTenantString(mongoCfg, f.key); err != nil {
				return fail(t, "invalid mongo tenant config", err)
			}
		}

		// IMPORTANT: If tenant database password is encrypted in master db config.
		if tenantDBPassPhraseKey != "" {
			password, err = aes.BeanAESDecrypt(tenantDBPassPhraseKey, password)
			if err != nil {
				return fail(t, "failed to decrypt mongo tenant password", err)
			}
		}

		// IMPORTANT - If a command or service wants to use a different `host` parameter for tenant database connection
		// then it's easy to do just by passing that parameter string name using `bean.TenantAlterDbHostParam`.
		// Therfore, `bean` will overwrite all host string in `TenantConnections`.`Connections` JSON.
		host, err := tenantHost(mongoCfg, tenantAlterDbHostParam)
		if err != nil {
			return fail(t, "invalid mongo tenant config", err)
		}

		ssl := sslConfigFromMap(mongoCfg)

		var closeDB func() error
		mongoConns[t.TenantID], mongoDBNames[t.TenantID], closeDB, err = connectMongoDB(
			userName, password, host, port, dbName,
			config.MaxConnectionPoolSize, config.MinConnectionPoolSize,
			config.ConnectTimeout, config.MaxConnectionLifeTime,
			config.Debug, ssl, logger,
		)
		if err != nil {
			return fail(t, "failed to connect mongo tenant database", err)
		}
		closers = append(closers, closeDB)
	}

	return mongoConns, mongoDBNames, closers, nil
}

// mongoDefaultConnectTimeout is used when `connectTimeout` is not set.
const mongoDefaultConnectTimeout = 10 * time.Second

// mongoDisconnectTimeout bounds the graceful disconnection (ending the sessions) on shutdown.
const mongoDisconnectTimeout = 10 * time.Second

// mongoConnString returns the connection string of the comma separated hosts, each with the default port unless it
// has its own (e.g. a replica set `a,b:27018`).
func mongoConnString(hostList, port string) string {
	var hosts []string
	for _, h := range strings.Split(hostList, ",") {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		if _, _, err := net.SplitHostPort(h); err != nil && port != "" {
			h = net.JoinHostPort(strings.Trim(h, "[]"), port)
		}
		hosts = append(hosts, h)
	}
	return "mongodb://" + strings.Join(hosts, ",")
}

func connectMongoDB(userName, password, host, port, dbName string,
	maxPoolSize, minPoolSize uint64,
	// maxConnIdleTime is `mongo.maxConnectionLifeTime` in env.json: the mongo driver has no maximum connection
	// lifetime, the closest option is the maximum idle time of a pooled connection.
	connectTimeout, maxConnIdleTime time.Duration,
	debug bool, ssl SSLConfig, logger *slog.Logger,
) (*mongo.Client, string, func() error, error) {

	connStr := mongoConnString(host, port)

	timeout := connectTimeout
	if timeout <= 0 {
		timeout = mongoDefaultConnectTimeout
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	opts := options.Client().
		ApplyURI(connStr).
		SetConnectTimeout(connectTimeout).
		SetMaxPoolSize(maxPoolSize).
		SetMinPoolSize(minPoolSize).
		SetMaxConnIdleTime(maxConnIdleTime)

	releaseTLS := noRelease
	if ssl.On {
		tlsConfig, release, err := newTLSConfig(ssl, hostsWithoutPort(host))
		if err != nil {
			return nil, "", noClose, err
		}
		releaseTLS = release
		opts.SetTLSConfig(tlsConfig)
	}

	if userName != "" && password != "" {
		credential := options.Credential{Username: userName, Password: password, AuthSource: dbName}
		opts.SetAuth(credential)
	}

	if logger == nil {
		logger = slog.Default()
	}

	// log monitor
	var logMonitor = event.CommandMonitor{
		Started: func(ctx context.Context, startedEvent *event.CommandStartedEvent) {
			logger.DebugContext(ctx, "mongo command started", "mongo_request_id", startedEvent.RequestID,
				"db", startedEvent.DatabaseName, "command", startedEvent.CommandName, "sql", startedEvent.Command.String())
		},
		Succeeded: func(ctx context.Context, succeededEvent *event.CommandSucceededEvent) {
			logger.DebugContext(ctx, "mongo command succeeded", "mongo_request_id", succeededEvent.RequestID,
				"command", succeededEvent.CommandName, "duration", succeededEvent.Duration)
		},
		Failed: func(ctx context.Context, failedEvent *event.CommandFailedEvent) {
			logger.DebugContext(ctx, "mongo command failed", "mongo_request_id", failedEvent.RequestID,
				"command", failedEvent.CommandName, "duration", failedEvent.Duration, "error", failedEvent.Failure)
		},
	}
	if debug {
		// cmd monitor set
		opts.SetMonitor(&logMonitor)
	}

	mdb, err := mongo.Connect(ctx, opts)
	if err != nil {
		releaseTLS()
		return nil, "", noClose, fmt.Errorf("failed to connect mongo database: %w", err)
	}

	disconnect := func() error {
		defer releaseTLS()
		// Not the connect ctx: it is canceled when this function returns.
		ctx, cancel := context.WithTimeout(context.Background(), mongoDisconnectTimeout)
		defer cancel()
		return mdb.Disconnect(ctx)
	}

	// Check the connection
	err = mdb.Ping(ctx, nil)
	if err != nil {
		return nil, "", noClose, errors.Join(fmt.Errorf("failed to ping mongo database: %w", err), disconnect())
	}

	return mdb, dbName, disconnect, nil
}
