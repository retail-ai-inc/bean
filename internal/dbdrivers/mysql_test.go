package dbdrivers

import (
	"errors"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
	"gorm.io/plugin/dbresolver"
)

func newMockMysqlDialector(t *testing.T) (gorm.Dialector, sqlmock.Sqlmock) {
	t.Helper()

	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	return mysql.New(mysql.Config{Conn: sqlDB, SkipInitializeWithVersion: true}), mock
}

func TestUseMysqlReplicasRouting(t *testing.T) {
	selectUsers := regexp.QuoteMeta("SELECT * FROM `users`")
	userRows := func() *sqlmock.Rows { return sqlmock.NewRows([]string{"id"}).AddRow(1) }

	tests := []struct {
		name      string
		toReplica bool
		expect    func(mock sqlmock.Sqlmock)
		run       func(db *gorm.DB) error
	}{
		{
			name:      "find goes to replica",
			toReplica: true,
			expect:    func(mock sqlmock.Sqlmock) { mock.ExpectQuery(selectUsers).WillReturnRows(userRows()) },
			run: func(db *gorm.DB) error {
				var rows []map[string]any
				return db.Table("users").Find(&rows).Error
			},
		},
		{
			name:      "raw select goes to replica",
			toReplica: true,
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT id FROM users")).WillReturnRows(userRows())
			},
			run: func(db *gorm.DB) error {
				var ids []int
				return db.Raw("SELECT id FROM users").Scan(&ids).Error
			},
		},
		{
			name: "write goes to master",
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET name = ?")).WithArgs("a").WillReturnResult(sqlmock.NewResult(0, 1))
			},
			run: func(db *gorm.DB) error {
				return db.Exec("UPDATE users SET name = ?", "a").Error
			},
		},
		{
			name:   "forced write clause reads from master",
			expect: func(mock sqlmock.Sqlmock) { mock.ExpectQuery(selectUsers).WillReturnRows(userRows()) },
			run: func(db *gorm.DB) error {
				var rows []map[string]any
				return db.Clauses(dbresolver.Write).Table("users").Find(&rows).Error
			},
		},
		{
			name: "locking read goes to master",
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM `users` FOR UPDATE")).WillReturnRows(userRows())
			},
			run: func(db *gorm.DB) error {
				var rows []map[string]any
				return db.Clauses(clause.Locking{Strength: "UPDATE"}).Table("users").Find(&rows).Error
			},
		},
		{
			name: "read inside transaction goes to master",
			expect: func(mock sqlmock.Sqlmock) {
				mock.ExpectBegin()
				mock.ExpectQuery(selectUsers).WillReturnRows(userRows())
				mock.ExpectCommit()
			},
			run: func(db *gorm.DB) error {
				return db.Transaction(func(tx *gorm.DB) error {
					var rows []map[string]any
					return tx.Table("users").Find(&rows).Error
				})
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			masterDialector, masterMock := newMockMysqlDialector(t)
			replicaDialector, replicaMock := newMockMysqlDialector(t)

			db, err := gorm.Open(masterDialector, &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
			require.NoError(t, err)
			require.NoError(t, useMysqlReplicas(db, []gorm.Dialector{replicaDialector}))

			if tt.toReplica {
				tt.expect(replicaMock)
			} else {
				tt.expect(masterMock)
			}

			require.NoError(t, tt.run(db))
			assert.NoError(t, masterMock.ExpectationsWereMet())
			assert.NoError(t, replicaMock.ExpectationsWereMet())
		})
	}
}

func TestReadHostsFromMap(t *testing.T) {
	tests := []struct {
		name    string
		cfg     map[string]any
		want    []string
		wantErr bool
	}{
		{name: "missing reads", cfg: map[string]any{}, want: nil},
		{name: "null reads", cfg: map[string]any{"reads": nil}, want: nil},
		{name: "empty reads", cfg: map[string]any{"reads": []any{}}, want: []string{}},
		{name: "multiple hosts", cfg: map[string]any{"reads": []any{"10.0.0.2", "10.0.0.3"}}, want: []string{"10.0.0.2", "10.0.0.3"}},
		{name: "not an array", cfg: map[string]any{"reads": "10.0.0.2"}, wantErr: true},
		{name: "non string host", cfg: map[string]any{"reads": []any{"10.0.0.2", 1.0}}, wantErr: true},
		{name: "empty host", cfg: map[string]any{"reads": []any{""}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := readHostsFromMap(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConnectMysqlDBRejectsInvalidReadHost(t *testing.T) {
	tests := []struct {
		name      string
		readHosts []string
	}{
		{name: "empty host", readHosts: []string{"10.0.0.2", ""}},
		{name: "invalid port", readHosts: []string{"10.0.0.2:3307", "10.0.0.3:abc"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, closeFn, err := connectMysqlDB("user", "pass", "127.0.0.1", "3306", "db", tt.readHosts,
				1, 1, 0, 0, false, SSLConfig{}, nil)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "index 1")
			assert.NoError(t, closeFn())
		})
	}
}

func TestMysqlDSNWithoutSSL(t *testing.T) {
	tests := []struct {
		name string
		host string
		want string
	}{
		{name: "ipv4", host: "10.0.0.2", want: "user:pass@tcp(10.0.0.2:3306)/db?parseTime=true&multiStatements=true"},
		{name: "hostname", host: "replica.internal", want: "user:pass@tcp(replica.internal:3306)/db?parseTime=true&multiStatements=true"},
		{name: "ipv6", host: "::1", want: "user:pass@tcp([::1]:3306)/db?parseTime=true&multiStatements=true"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn, err := mysqlDSN("user", "pass", tt.host, "3306", "db", SSLConfig{})

			require.NoError(t, err)
			assert.Equal(t, tt.want, dsn)
		})
	}
}

func TestParseReadAddr(t *testing.T) {
	tests := []struct {
		name     string
		readHost string
		want     mysqlAddr
		wantErr  bool
	}{
		{name: "host uses default port", readHost: "10.0.0.2", want: mysqlAddr{host: "10.0.0.2", port: "3306"}},
		{name: "hostname uses default port", readHost: "replica.internal", want: mysqlAddr{host: "replica.internal", port: "3306"}},
		{name: "host with port", readHost: "127.0.0.1:3307", want: mysqlAddr{host: "127.0.0.1", port: "3307"}},
		{name: "bracketed ipv6 uses default port", readHost: "[::1]", want: mysqlAddr{host: "::1", port: "3306"}},
		{name: "bracketed ipv6 with port", readHost: "[::1]:3307", want: mysqlAddr{host: "::1", port: "3307"}},
		{name: "empty", readHost: "", wantErr: true},
		{name: "missing host", readHost: ":3307", wantErr: true},
		{name: "empty port", readHost: "10.0.0.2:", wantErr: true},
		{name: "non numeric port", readHost: "10.0.0.2:abc", wantErr: true},
		{name: "port out of range", readHost: "10.0.0.2:70000", wantErr: true},
		{name: "unbracketed ipv6", readHost: "::1", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseReadAddr(tt.readHost, "3306")
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCloseAll(t *testing.T) {
	errFirst := errors.New("first")
	errThird := errors.New("third")
	var calls int

	err := closeAll([]func() error{
		func() error { calls++; return errFirst },
		func() error { calls++; return nil },
		func() error { calls++; return errThird },
	})

	assert.Equal(t, 3, calls, "every closer must run even after a failure")
	assert.ErrorIs(t, err, errFirst)
	assert.ErrorIs(t, err, errThird)
	assert.NoError(t, closeAll(nil))
}
